# CRC Caddy Image

The CRC Caddy Image is used in the ephemeral environments to emulate the authentications used on ConsoleDot. It is deployed via the [Clowder](https://github.com/RedHatInsights/clowder) operator. The functionality is provided as a Caddy plugin.

This Caddy image presents a simple reverse proxy to an individual service. The intention for it is for the container to be run inside the same pod as the service it provides authentication for.

## Configuration
The Caddy CRC plugin has three configuration options, an example of which is shown below:

```
:8080 {
    log
    tls internal

    crcauth {
        output stdout
        bop http://my-bop-server
        whitelist /api/unauth,/api/unauth-dir/file
    }
    reverse_proxy 127.0.0.1:{$CADDY_PORT}
}
```

* `output` - defines where the output of the log stream goes, either `stdout` or `stderr`
* `bop` - defines the host:port for the BOP server, or MBOP in the case of ephemeral environments
  * For Basic Auth, BOP provides an endpoint to check the username/password
  * For JWT Auth, BOP provides the JWT public certificate that is used to validate JWT tokens which contain user identity info
* `whitelist` - defines a comma separated list of API paths that do NOT require authentication

## crcauthlib
This repo houses the repository for the caddy plugin itself. The actual code that performs all the auth operations is housed in the [crcauthlib repo](https://github.com/redhatinsights/crcauthlib/).
