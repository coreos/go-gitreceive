go-gitreceive
=======

`go-gitreceive` is meant to handle the following tasks:

1. init - add the "git" user
2. upload-key - uploads SSH keys for users so they can push git repositories
3. run - runs a command and initializes the git repository if it doesn't exist
4. hook - runs a pre-receive hook in order to deploy an application


### Purpose

It's meant to be a Go implementation of [progrium/gitreceive](https://github.com/progrium/gitreceive) which doesn't rely on bash and can integrate into third party systems and APIs.

It's currently a drop in replacement for the bash version of `gitreceive`.

### Requirements

You need to have Go 1.1 installed to build this. You also need to have `git` and `sshd` installed for this tool to work.

```
git clone https://github.com/coreos/go-gitreceive.git
cd go-gitreceive
go build
```

### Usage


#### Add the git system user:

This adds a system user called 'git' with a home directory `/home/git`.

```
$ sudo go-gitreceive init
```

### Create an user

This uploads an SSH key from another system. You need to specify the username, too.
The username specified at the end won't be a system user, but it'll be associated to your public key.

```
$ cat ~/.ssh/id_rsa.pub | ssh yoursystemuser@server "sudo go-gitreceive upload-key yourdesireduser"
```

### Add a remote to an existing repository

This will add a remote to an existing repository. The remote repository will be created automatically the first time you push.

```
$ git remote add test git@server.com:testproject.git
```

### Push

This will push to the remote git repository and create it.
The example below is an example of what's displayed when using go-gitreceive with dokku to deploy a go web application.

The `go-gitreceive` binary has to be named `gitreceive` in order to work as drop in replacement to `gitreceive`.

```
$ git push test master
Counting objects: 5, done.
Delta compression using up to 2 threads.
Compressing objects: 100% (3/3), done.
Writing objects: 100% (3/3), 292 bytes, done.
Total 3 (delta 2), reused 0 (delta 0)
remote: -----> Building gotest ...
remote:        Go app detected
remote: -----> Installing Go 1.1.1... done
remote:        Installing Virtualenv... done
remote:        Installing Mercurial... done
remote:        Installing Bazaar... done
remote: -----> Running: go get -tags heroku ./...
remote: -----> Discovering process types
remote:        Procfile declares types -> web, worker
remote: -----> Build complete!
remote: -----> Deploying got ...
remote: -----> Application deployed:
remote:        http://gotest.app.dev
remote:
To git@precise64:got
   95d3fe9..c754cb6  master -> master

```
