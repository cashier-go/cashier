# cashier

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

## How it works:
The user wishes to ssh to a production machine.

They visit the CA site (e.g. https://sshca.exampleorg.com) in a browser and authenticate.

The site shows a page with a token which the user copies.

The user runs a local command which generates a new ssh key-pair in memory and requests the token from the user.

The token is sent to the CA along with the ssh public key.

The CA verifies the token and signs the public key with the signing key and returns the signed certificate.

The command on the user's machine receives the certificate and loads it and the previously generated private key into the ssh agent.

The user can now ssh to the production machine, and continue to ssh to any machine that trusts the CA signing key until the certificate is revoked or expires or is removed from the agent.

# Usage
Cashier comes in two parts, a [cli](cmd/cashier) and a [server](cmd/cashierd).
The client is configured using command-line flags.
The server is configured using a JSON configuration file - [example](exampleconfig.json).

For the server you need the following:
- A new ssh private key. Generate one in the usual way using `ssh-keygen -f ssh_ca` - this is your CA signing key. At this time Cashier supports RSA, ECDSA and Ed25519 keys. *Important* This key should be kept safe - *ANY* ssh key signed with this key will be able to access your machines.
- Google OAuth credentials which you can generate at the [Google Developers Console](https://console.developers.google.com). You also need to set the callback URL here.

# Quick start
## Installation using Go tools
1. Use the Go tools to install cashier. The binaries `cashierd` and `cashier` will be installed in your $GOPATH.
```
go get github.com/cashier/cmd/...
```
2. Create a signing key with `ssh-keygen` and a [config.json](exampleconfig.json)
3. Run the cashier server with `cashierd` and the cli with `cashier`.

## Using docker
1. Create a signing key with `ssh-keygen` and a [config.json](exampleconfig.json)
2. Run
```
docker run -it --rm -p 10000:10000 --name cashier -v $(pwd):/cashier nsheridan/cashier
```

# Requirements
## Server
Go 1.5+. May work with earlier versions.

## Client
OpenSSH 5.6 or newer.
A working SSH agent.
I have only tested this on Linux & OSX.

# Configuration
Configuration is divided into three sections: `server`, `auth`, and `ssh`.

### server
- `use_tls` : boolean. If set `tls_key` and `tls_cert` are required.
- `tls_key` : string. Path to the TLS key.
- `tls_cert` : string. Path to the TLS cert.
- `port` : int. Port to listen on.
- `cookie_secret`: string. Authentication key for the session cookie.
- `template_dir`: string. Path to html template directory. At present only 'token.html' is required.

### auth
- `provider` : string. Name of the oauth provider. At present the only valid value is "google".
- `oauth_client_id` : string. Oauth Client ID.
- `oauth_client_secret` : string. Oauth secret.
- `oauth_callback_url` : string. URL that the Oauth provider will redirect to after user authorisation. The path is hardcoded to `"/auth/callback"` in the source.
- `provider_opts` : object. Additional options for the provider.

#### Provider-specific options

Oauth providers can support provider-specific options - e.g. to ensure organization membership.
Options are set in the `provider_opts` hash.

Example:

```
"auth": {
  "provider": "google",
  "provider_opts" : {
    "domain": "example.com"
  }
}
```

| Provider |       Option | Notes                                                                                                                                  |
|---------:|-------------:|----------------------------------------------------------------------------------------------------------------------------------------|
| Google   |       domain | If this is unset then any gmail user can obtain a token.                                                                               |
| Github   | organization | If this is unset then any GitHub user can obtain a token. The oauth client and secrets should be issued by the specified organization. |

Supported options:

### ssh
- `signing_key`: string. Path to the signing ssh private key you created earlier.
- `additional_principals`: array of string. By default certificates will have one principal set - the username portion of the requester's email address. If `additional_principals` is set, these will be added to the certificate e.g. if your production machines use shared user accounts.
- `max_age`: string. If set the server will not issue certificates with an expiration value longer than this, regardless of what the client requests. Must be a valid Go [`time.Duration`](https://golang.org/pkg/time/#ParseDuration) string.
- `permissions`: array of string. Actions the certificate can perform. See the [`-O` option to `ssh-keygen(1)`](http://man.openbsd.org/OpenBSD-current/man1/ssh-keygen.1) for a complete list.

## Configuring ssh
The client needs no special configuration, just a running ssh-agent.
The ssh server needs to trust the public part of the CA signing key. Add something like the following to your sshd_config:
```
TrustedUserCAKeys /etc/ssh/ca.pub
```
where `/etc/ssh/ca.pub` contains the public part of your signing key.

## Future Work

- Host certificates - only user certificates are supported at present.
