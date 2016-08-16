package testdata

var ServerConfig = []byte(`
	server {
		use_tls = true
		tls_key = "server.key"
		tls_cert = "server.crt"
		address = "127.0.0.1"
		port = 443
		cookie_secret = "supersecret"
	}
	auth {}
	ssh {}
`)

var AuthConfig = []byte(`
	auth {
		provider = "google"
		oauth_client_id = "client_id"
		oauth_client_secret = "secret"
		oauth_callback_url = "https://sshca.example.com/auth/callback"
		provider_opts {
			domain = "example.com"
		}
	}
	server {}
	ssh {}
`)

var SSHConfig = []byte(`
	ssh {
		signing_key = "signing_key"
		additional_principals = ["ec2-user", "ubuntu"]
		max_age = "720h"
		permissions = ["permit-pty", "permit-X11-forwarding", "permit-port-forwarding", "permit-user-rc"]
	}
	auth {}
	server {}
`)
