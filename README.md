



Converting the certificate

First you have to export the certificate that is stored on your OS X
Server. Do this by opening Keychain.app and finding the certificate in
the System directory

```
openssl pkcs12 -in ExportedCertificate.p12 -nocerts -nodes -out key.pem
openssl pkcs12 -in ExportedCertificate.p12 -clcerts -nokeys -out certificate.pem
```

You can test if the certificate and key are correct by making a
connection to the apple push notifications gateway:

```
openssl s_client -connect gateway.push.apple.com:2195 -cert certificate.pem -key key.pem
```

The connection may close but check if you see `Verify return code: 0 (ok)` appear.

You now have your exported certificate and private key stored in two
separate PEM encoded files that can be used by the xapsd daemon. By
default the daemon will look for these files in the following location:

 * `/etc/xapsd/certificate.pem`
 * `/etc/xapsd/key.pem`
