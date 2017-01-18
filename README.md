# Cashier

[![Build Status](https://travis-ci.org/nsheridan/cashier.svg?branch=master)](https://travis-ci.org/nsheridan/cashier)

- [Cashier](#cashier)
	- [How it works](#how-it-works)
- [Quick start](#quick-start)
	- [Installation using Go tools](#installation-using-go-tools)
	- [Using docker](#using-docker)
- [Requirements](#requirements)
	- [Server](#server)
	- [Client](#client)
- [Configuration](#configuration)
	- [server](#server-1)
		- [database](#database)
		- [datastore](#datastore) [DEPRECATED]
	- [auth](#auth)
		- [Provider-specific options](#provider-specific-options)
	- [ssh](#ssh)
	- [aws](#aws)
	- [vault](#vault)
- [Usage](#usage)
	- [Using cashier](#using-cashier)
	- [Configuring SSH](#configuring-ssh)
	- [Revoking certificates](#revoking-certificates)
- [Future Work](#future-work)
- [Contributing](#contributing)

Cashier is a SSH Certificate Authority (CA).

OpenSSH supports authentication using SSH certificates.
Certificates contain a public key, identity information and are signed with a standard SSH key.

Unlike ssh keys, certificates can contain additional information:
- Which user(s) may use the certificate
- When the certificate is valid from
- When the certificate expires
- Permissions

Other benefits of certificates:
-  Unlike keys certificates don't need to be distributed to every machine - the sshd just needs to trust the key that signed the certificate.
- This also works for host keys - machines can get new (signed) host certs which clients can authenticate. No more blindly typing "yes".
- Certificates can be revoked.

See also the `CERTIFICATES` [section](http://man.openbsd.org/OpenBSD-current/man1/ssh-keygen.1#CERTIFICATES) of `ssh-keygen(1)`

## How it works
The user wishes to ssh to a production machine.

They run a command which opens the CA site (e.g. https://sshca.exampleorg.com) in a browser and they login.

The CA displays a token which the user copies.

The user provides the token to the client. The client generates a new ssh key-pair.

The client sends the ssh public key to the CA along with the token.

The CA verifies the token and signs the public key with the signing key and returns the signed certificate.

The client receives the certificate and loads it and the private key into the ssh agent.

The user can now ssh to the production machine, and continue to ssh to any machine that trusts the CA signing key until the certificate is revoked or expires or is removed from the agent.

# Quick start
## Installation using Go tools
1. Use the Go tools to install cashier. The binaries `cashierd` and `cashier` will be installed in your $GOPATH.
```
go get -u github.com/nsheridan/cashier/cmd/cashier
go get -u github.com/nsheridan/cashier/cmd/cashierd
```
2. Create a signing key with `ssh-keygen` and a [cashierd.conf](example-server.conf)
3. Run the cashier server with `cashierd` and the cli with `cashier`.

## Using docker
1. Create a signing key with `ssh-keygen` and a [cashierd.conf](example-server.conf)
2. Run
```
docker run -it --rm -p 10000:10000 --name cashier -v $(pwd):/cashier nsheridan/cashier
```

# Requirements
## Server
Go 1.7 or later. May work with earlier versions.

## Client
- OpenSSH 5.6 or newer.
- A working SSH agent.

Note: I have only tested this on Linux & OSX.

# Configuration
Configuration is divided into different sections: `server`, `auth`, `ssh`, and `aws`.

## A note on files:
For any option that takes a file path as a parameter (e.g. SSH signing key, TLS key, TLS cert), the path can be one of:

- A relative or absolute filesystem path e.g. `/data/ssh_signing_key`, `tls/server.key`.
- An AWS S3 bucket + object path starting with `/s3/` e.g. `/s3/my-bucket/ssh_signing_key`. You should add an [aws](#aws) config as needed.
- A Google GCS bucket + object path starting with `/gcs/` e.g. `/gcs/my-bucket/ssh_signing_key`.
- A [Vault](https://www.vaultproject.io) path + key starting with `/vault/` e.g. `/vault/secret/cashier/ssh_signing_key`. You should add a [vault](#vault) config as needed.

Exception to this: the `http_logfile` option **ONLY** writes to local files.

## server
- `use_tls` : boolean. If this is set then either `tls_key` and `tls_cert` are required, or `letsencrypt_servername` is required.
- `tls_key` : string. Path to the TLS key. See the [note](#a-note-on-files) on files above.
- `tls_cert` : string. Path to the TLS cert. See the [note](#a-note-on-files) on files above.
- `letsencrypt_servername`: string. If set will request a certificate from LetsEncrypt. This should match the expected FQDN of the server.
- `letsencrypt_cachedir`: string. Directory to cache the LetsEncrypt certificate. See the [note](#a-note-on-files) on files above.
- `address` : string. IP address to listen on. If unset the server listens on all addresses.
- `port` : int. Port to listen on.
- `user` : string. User to which the server drops privileges to.
- `cookie_secret`: string. Authentication key for the session cookie. This can be a secret stored in a [vault](https://www.vaultproject.io/) using the form `/vault/path/key` e.g. `/vault/secret/cashier/cookie_secret`.
- `csrf_secret`: string. Authentication key for CSRF protection. This can be a secret stored in a [vault](https://www.vaultproject.io/) using the form `/vault/path/key` e.g. `/vault/secret/cashier/csrf_secret`.
- `http_logfile`: string. Path to the HTTP request log. Logs are written in the [Common Log Format](https://en.wikipedia.org/wiki/Common_Log_Format). The only valid destination for logs is a local file path.
- `datastore`: string. Datastore connection string. See [Datastore](#datastore).

### database

The database is used to record issued certificates for audit and revocation purposes.

- `type` : string. One of `mongo`, `mysql`, `sqlite` or `mem`. Default: `mem`.
- `address` : string. (`mongo` and `mysql` only) Hostname and optional port of the database server. For MongoDB replica sets separate multiple entries with commas.
- `username` : string. Database username.
- `password` : string. Database password. This can be a secret stored in a [vault](https://www.vaultproject.io/) using the form `/vault/path/key` e.g. `/vault/secret/cashier/mysql_password`.
- `filename` : string. (`sqlite` only). Path to sqlite database.

Examples:
```
server {
  database {
    type = "mysql"
    address = "my-db-host.corp"
    username = "user"
    password = "passwd"
  }

  database {
    type = "mongo"
    address = "mongo-host1.corp:27017,mongo-host2.corp:27018"
    username = "user"
    password = "passwd"
  }

  database {
    type = "mem"
  }

  database {
    type = "sqlite"
    filename = "/data/cashier.db"
  }
}
```

Prior to using MySQL, MongoDB or SQLite you need to create the database and tables using [one of the provided files](db).  
e.g. `mysql < db/seed.sql` or `mongo db/seed.js`.  
Obviously you should setup a role user for running in prodution.

### datastore

## The datastore option is deprecated. Use the [database](#database) option instead

~~Datastores contain a record of issued certificates for audit and revocation purposes. The connection string is of the form `engine:username:password:host[:port]`.~~

~~Supported database providers: `mysql`, `mongo`, `sqlite` and `mem`.~~

~~`mem` is an in-memory database intended for testing and takes no additional config options.~~  
~~`mysql` is the MySQL database and accepts `username`, `password` and `host` arguments. Only `username` and `host` arguments are required. `port` is assumed to be 3306 unless otherwise specified.~~  
~~`mongo` is MongoDB and accepts `username`, `password` and `host` arguments. All arguments are optional and multiple hosts can be specified using comma-separated values: `mongo:dbuser:dbpasswd:host1,host2`.~~  
~~`sqlite` is the SQLite database and accepts a `path` argument.~~

~~If no datastore is specified the `mem` store is used by default.~~

~~Examples:~~

```
server {
  datastore = "mem"  # use the in-memory database.
  datastore = "mysql:root::localhost"  # mysql running on localhost with the user 'root' and no password.
  datastore = "mysql:cashier:PaSsWoRd:mydbprovider.example.com:5150"  # mysql running on a remote host on port 5150
  datastore = "mongo:cashier:PaSsWoRd:mydbprovider.example.com:27018"  # mongo running on a remote host on port 27018
  datastore = "mongo:cashier:PaSsWoRd:server1.example.com:27018,server2.example.com:27018"  # mongo running on multiple servers on port 27018
  datastore = "sqlite:/data/certs.db"
}
```

## auth
- `provider` : string. Name of the oauth provider. Valid providers are currently "google" and "github".
- `oauth_client_id` : string. Oauth Client ID. This can be a secret stored in a [vault](https://www.vaultproject.io/) using the form `/vault/path/key` e.g. `/vault/secret/cashier/oauth_client_id`.
- `oauth_client_secret` : string. Oauth secret. This can be a secret stored in a [vault](https://www.vaultproject.io/) using the form `/vault/path/key` e.g. `/vault/secret/cashier/oauth_client_secret`.
- `oauth_callback_url` : string. URL that the Oauth provider will redirect to after user authorisation. The path is hardcoded to `"/auth/callback"` in the source.
- `provider_opts` : object. Additional options for the provider.
- `users_whitelist` : array of strings. Optional list of whitelisted usernames. If missing, all users of your current domain/organization are allowed to authenticate against cashierd. For Google auth a user is an email address. For GitHub auth a user is a GitHub username.

### Provider-specific options

Oauth providers can support provider-specific options - e.g. to ensure organization membership.
Options are set in the `provider_opts` hash.

Example:

```
auth {
  provider = "google"
  provider_opts {
    domain = "example.com"
  }
}
```

Supported options:


| Provider |       Option | Notes                                                                                                                                  |
|---------:|-------------:|----------------------------------------------------------------------------------------------------------------------------------------|
| Google   |       domain | If this is unset then you must whitelist individual email addresses using `users_whitelist`.                                           |
| Github   | organization | If this is unset then you must whitelist individual users using `users_whitelist`. The oauth client and secrets should be issued by the specified organization. |

## ssh
- `signing_key`: string. Path to the signing ssh private key you created earlier. See the [note](#a-note-on-files) on files above.
- `additional_principals`: array of string. By default certificates will have one principal set - the username portion of the requester's email address. If `additional_principals` is set, these will be added to the certificate e.g. if your production machines use shared user accounts.
- `max_age`: string. If set the server will not issue certificates with an expiration value longer than this, regardless of what the client requests. Must be a valid Go [`time.Duration`](https://golang.org/pkg/time/#ParseDuration) string.
- `permissions`: array of string. Specify the actions the certificate can perform. See the [`-O` option to `ssh-keygen(1)`](http://man.openbsd.org/OpenBSD-current/man1/ssh-keygen.1) for a complete list. e.g. `permissions = ["permit-pty", "permit-port-forwarding", force-command=/bin/ls", "source-address=192.168.0.0/24"]`

## aws
AWS configuration is only needed for accessing signing keys stored on S3, and isn't totally necessary even then.  
The S3 client can be configured using any of [the usual AWS-SDK means](https://github.com/aws/aws-sdk-go/wiki/configuring-sdk) - environment variables, IAM roles etc.  
It's strongly recommended that signing keys stored on S3 be locked down to specific IAM roles and encrypted using KMS.  

- `region`: string. AWS region the bucket resides in, e.g. `us-east-1`.
- `access_key`: string. AWS Access Key ID. This can be a secret stored in a [vault](https://www.vaultproject.io/) using the form `/vault/path/key` e.g. `/vault/secret/cashier/aws_access_key`.
- `secret_key`: string. AWS Secret Key. This can be a secret stored in a [vault](https://www.vaultproject.io/) using the form `/vault/path/key` e.g. `/vault/secret/cashier/aws_secret_key`.

## vault
Vault support is currently a work-in-progress.

- `address`: string. URL to the vault server.
- `token`: string. Auth token for the vault.

# Usage
Cashier comes in two parts, a [cli](cmd/cashier) and a [server](cmd/cashierd).  
The server is configured using a HCL configuration file - [example](example-server.conf).

For the server you need the following:
- A new ssh private key. Generate one in the usual way using `ssh-keygen -f ssh_ca` - this is your CA signing key. At this time Cashier supports RSA, ECDSA and Ed25519 keys. *Important* This key should be kept safe - *ANY* ssh key signed with this key will be able to access your machines.
- OAuth (Google or GitHub) credentials. You may also need to set the callback URL when creating these.

## Using cashier
Once the server is up and running you'll need to configure your client.  
The client is configured using either a [HCL](https://github.com/hashicorp/hcl) configuration file - [example](example-client.conf) - or command-line flags.  

Running the `cashier` cli tool will open a browser window at the configured CA address.
The CA will redirect to the auth provider for authorisation, and redirect back to the CA where the access token will printed.  
Copy the access token. In the terminal where you ran the `cashier` cli paste the token at the prompt.  
The client will then generate a new ssh key-pair and send the public part to the server (along with the access token).  
Once signed the client will install the key and signed certificate in your ssh agent. When the certificate expires it will be removed automatically from the agent.

## Configuring SSH
The ssh client needs no special configuration, just a running ssh-agent.  
The ssh server needs to trust the public part of the CA signing key. Add something like the following to your sshd_config:
```
TrustedUserCAKeys /etc/ssh/ca.pub
```
where `/etc/ssh/ca.pub` contains the public part of your signing key.

If you wish to use certificate revocation you need to set the `RevokedKeys` option in sshd_config - see the next section.

## Revoking certificates
When a certificate is signed a record is kept in the configured datastore. You can view issued certs at `http(s)://<ca url>/admin/certs` and also revoke them.  
The revocation list is served at `http(s)://<ca url>/revoked`. To use it your sshd_config must have `RevokedKeys` set:
```
RevokedKeys /etc/ssh/revoked_keys
```
See the [`RevokedKeys` option in the sshd_config man page](http://man.openbsd.org/OpenBSD-current/man5/sshd_config) for more.  
Keeping the revoked list up to date can be done with a cron job like:
```
*/10 * * * * * curl -s -o /etc/ssh/revoked_keys https://sshca.example.com/revoked
```

Remember that the `revoked_keys` file **must** exist and **must** be readable by the sshd or else all ssh authentication will fail.

# Future Work

- Host certificates - only user certificates are supported at present.

# Contributing
Pull requests are welcome but forking Go repos can be a pain. [This is a good guide to forking and creating pull requests for Go projects](https://splice.com/blog/contributing-open-source-git-repositories-go/).  
Dependencies are vendored with [govendor](https://github.com/kardianos/govendor).
