package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

const gitUser = "git"
const gitHome = "/home/git"

const prereceiveScript = `
#!/bin/bash
cat | %s hook
`

func getExitCode(err error) (int, error) {
	exitCode := 0
	if exiterr, ok := err.(*exec.ExitError); ok {
		if procExit := exiterr.Sys().(syscall.WaitStatus); ok {
			return procExit.ExitStatus(), nil
		}
	}
	return exitCode, fmt.Errorf("failed to get exit code")
}

func runCommandWithOutput(cmd *exec.Cmd) (output string, exitCode int, err error) {
	exitCode = 0
	out, err := cmd.CombinedOutput()
	if err != nil {
		var exiterr error
		if exitCode, exiterr = getExitCode(err); exiterr != nil {
			// TODO: Fix this so we check the error's text.
			// we've failed to retrieve exit code, so we set it to 127
			exitCode = 127
		}
	}
	output = string(out)
	return
}

func runCommand(cmd *exec.Cmd) (exitCode int, err error) {
	exitCode = 0
	err = cmd.Run()
	if err != nil {
		var exiterr error
		if exitCode, exiterr = getExitCode(err); exiterr != nil {
			// TODO: Fix this so we check the error's text.
			// we've failed to retrieve exit code, so we set it to 127
			exitCode = 127
		}
	}
	return
}

func addGitUser(homeDirectory, gitUsername string) {
	userAddCmd := exec.Command("useradd", "-d", homeDirectory, gitUsername)
	if out, _, err := runCommandWithOutput(userAddCmd); err != nil {
		fmt.Printf("failed to execute add user %s\n", out)
		os.Exit(1)
	}

	sshDir := fmt.Sprintf("%s/.ssh", homeDirectory)
	addSshDirCmd := exec.Command("mkdir", "-p", sshDir)
	if _, _, err := runCommandWithOutput(addSshDirCmd); err != nil {
		fmt.Printf("failed to create the .ssh directory\n")
		os.Exit(1)
	}

	authorizedKeysFilename := fmt.Sprintf("%s/.ssh/authorized_keys", homeDirectory)
	authorizedKeys, err := os.OpenFile(authorizedKeysFilename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		fmt.Printf("failed to open authorized_keys %s\n", authorizedKeysFilename)
		os.Exit(1)
	}
	authorizedKeys.Close()

	changeOWnership := exec.Command("chown", "-R", gitUsername, homeDirectory)
	if _, _, err := runCommandWithOutput(changeOWnership); err != nil {
		fmt.Printf("failed to change ownership\n")
		os.Exit(1)
	}
	fmt.Printf("Created receiver script in %s for user '%s'.\n", homeDirectory, gitUsername)
}

func uploadKey(homeDirectory, gitreceivePath, username string) {
	inputRawKey, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		fmt.Printf("failed to read key from stdin\n")
		os.Exit(1)
	}
	key := string(inputRawKey)

	// ssh-keygen doesn't read from pipes and sometimes /dev/stdin doesn't work
	// create a temporary file to write the key to it
	createTmpFile := exec.Command("mktemp")
	tmpFilename, _, err := runCommandWithOutput(createTmpFile)
	if err != nil {
		fmt.Printf("failed to create a temporary file\n")
		os.Exit(1)
	}

	tmpFile, err := os.OpenFile(tmpFilename, os.O_RDWR|os.O_CREATE, 0770)
	if err != nil {
		fmt.Printf("failed to open temporary file %s\n", tmpFilename)
		os.Exit(1)
	}

	if _, err := tmpFile.WriteString(key); err != nil {
		fmt.Printf("failed to write key to temporary file %s\n", tmpFilename)
		tmpFile.Close()
		os.Exit(1)
	}
	tmpFile.Close()

	getFingerprint := exec.Command("ssh-keygen", "-lf", tmpFilename)
	rawFingerprint, _, err := runCommandWithOutput(getFingerprint)
	if err != nil {
		fmt.Printf("failed to read key %s %s\n", err, rawFingerprint)
		os.Exit(1)
	}

	splitFingerprint := strings.Split(rawFingerprint, " ")
	if len(splitFingerprint) < 2 {
		fmt.Printf("fingerprint seems invalid: %s\n", rawFingerprint)
		os.Exit(1)
	}

	if err := os.Remove(tmpFilename); err != nil {
		fmt.Printf("failed to remove the temporary file %s\n", tmpFilename)
		os.Exit(1)
	}

	authorizedKeysFilename := fmt.Sprintf("%s/.ssh/authorized_keys", homeDirectory)
	authorizedKeys, err := os.OpenFile(authorizedKeysFilename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		fmt.Printf("failed to open authorized_keys %s\n", authorizedKeysFilename)
		os.Exit(1)
	}
	fingerprint := splitFingerprint[1]

	keyPrefixTemplate := `command=%s run %s %s,no-agent-forwarding,no-pty,no-user-rc,no-X11-forwarding,no-port-forwarding`
	keyPrefix := fmt.Sprintf(keyPrefixTemplate, gitreceivePath, username, fingerprint)
	authorizedKeyEntry := fmt.Sprintf("%s %s", keyPrefix, key)

	if _, err := authorizedKeys.WriteString(authorizedKeyEntry); err != nil {
		fmt.Printf("failed to add key to authorized_keys")
		authorizedKeys.Close()
		os.Exit(1)
	}
	authorizedKeys.Close()

	fmt.Printf("%s\n", fingerprint)
}

func run(gitHome, receiveUser, receiveFingerprint, gitreceivePath string) {
	originalSSHCommand := os.Getenv("SSH_ORIGINAL_COMMAND")
	if len(originalSSHCommand) == 0 {
		fmt.Printf("SSH_ORIGINAL_COMMAND is undefined\n")
		os.Exit(1)
	}

	splitOriginalSSHCommand := strings.Split(originalSSHCommand, " ")
	if len(splitOriginalSSHCommand) < 2 {
		fmt.Printf("SSH_ORIGINAL_COMMAND is too short %s\n", originalSSHCommand)
		os.Exit(1)
	}

	repoRaw := splitOriginalSSHCommand[1]
	for key, value := range splitOriginalSSHCommand {
		splitOriginalSSHCommand[key] = strings.Trim(value, "'")
	}

	repo := strings.Trim(repoRaw, "'")

	repoPath := fmt.Sprintf("%s/%s", gitHome, repo)

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		if err := os.Mkdir(repoPath, 0770); err != nil {
			fmt.Printf("failed to create repo directory\n")
			os.Exit(1)
		}
		initRepo := exec.Command("git", "init", "--bare")
		initRepo.Dir = repoPath
		if _, _, err := runCommandWithOutput(initRepo); err != nil {
			fmt.Printf("failed to initialize repository\n")
			os.Exit(1)
		}
	}

	prereceiveHookPath := fmt.Sprintf("%s/hooks/pre-receive", repoPath)
	prereceiveHook, err := os.OpenFile(prereceiveHookPath, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0770)
	if err != nil {
		fmt.Printf("failed to open repo pre-receive hook script\n")
		os.Exit(1)
	}

	renderedPrereceiveScript := fmt.Sprintf(prereceiveScript, gitreceivePath)
	if _, err := prereceiveHook.WriteString(renderedPrereceiveScript); err != nil {
		prereceiveHook.Close()
		fmt.Printf("failed to write repo pre-receive hook script\n")
		os.Exit(1)
	}
	prereceiveHook.Close()
	env := os.Environ()

	receiveUserEnv := fmt.Sprintf("RECEIVE_USER=%s", receiveUser)
	receiveFingerprintEnv := fmt.Sprintf("RECEIVE_FINGERPRINT=%s", receiveFingerprint)
	receiveRepo := fmt.Sprintf("RECEIVE_REPO=%s", repo)
	githomeEnv := fmt.Sprintf("GITHOME=%s", gitHome)

	originalSSHCmd := exec.Command(splitOriginalSSHCommand[0], splitOriginalSSHCommand[1:]...)
	originalSSHCmd.Dir = gitHome
	originalSSHCmd.Env = append(env, receiveUserEnv, receiveFingerprintEnv, receiveRepo, githomeEnv)
	originalSSHCmd.Stdout = os.Stdout
	originalSSHCmd.Stdin = os.Stdin
	originalSSHCmd.Stderr = os.Stderr

	if exitCode, err := runCommand(originalSSHCmd); err != nil {
		fmt.Println(err)
		os.Exit(exitCode)
	}
}

func hook() {
	lineReader := bufio.NewScanner(os.Stdin)

	receiveUser := os.Getenv("RECEIVE_USER")
	receiveFingerprint := os.Getenv("RECEIVE_FINGERPRINT")
	receiveRepo := os.Getenv("RECEIVE_REPO")
	githomeEnv := os.Getenv("GITHOME")

	receiverPath := fmt.Sprintf("%s/receiver", githomeEnv)

	for lineReader.Scan() {
		line := lineReader.Text()
		args := strings.Split(line, " ")
		//oldRev := args[0]
		newRev := args[1]
		refName := args[2]

		if refName != "refs/heads/master" {
			continue
		}

		gitArchiver := exec.Command("git", "archive", newRev)
		receiver := exec.Command(receiverPath, receiveRepo, newRev, receiveUser, receiveFingerprint)
		receiver.Stdin, _ = gitArchiver.StdoutPipe()
		receiver.Stdout = os.Stdout
		err := receiver.Start()
		if err != nil {
			fmt.Printf("push denied - failed to start receiver for %s %s", newRev, err)
			os.Exit(1)
		}
		err = gitArchiver.Run()
		if err != nil {
			fmt.Printf("push denied - failed to run git archiver for %s %s", newRev, err)
			os.Exit(1)
		}
		err = receiver.Wait()
		if err != nil {
			fmt.Printf("push denied - receiver failed to exit cleanly for %s %s", newRev, err)
			os.Exit(1)
		}
	}
}

func main() {
	if len(os.Args) == 1 {
		return
	}

	gitreceivePath := os.Args[0]

	switch os.Args[1] {
	case "init":
		addGitUser(gitHome, gitUser)
	case "upload-key":
		uploadKey(gitHome, gitreceivePath, os.Args[2])
	case "run":
		run(gitHome, os.Args[2], os.Args[3], gitreceivePath)
	case "hook":
		hook()
	}

}
