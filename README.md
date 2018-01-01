[![Build Status](https://travis-ci.org/st3fan/dovecot-xaps-daemon.svg)](https://travis-ci.org/st3fan/dovecot-xaps-daemon)

iOS Push Email for Dovecot
==========================

What is this?
-------------

This project, together with the [dovecot-xaps-plugin](https://github.com/st3fan/dovecot-xaps-plugin) project, will enable push email for iOS devices that talk to your Dovecot 2.0.x IMAP server. This is specially useful for people who are migrating away from running email services on OS X Server and want to keep the Push Email ability.

> Please note that it is not possible to use this project without legally owning a copy of OS X Server. You can purchase OS X Server on the [Mac App Store](https://itunes.apple.com/ca/app/os-x-server/id714547929?mt=12) or download it for free if you are a registered Mac or iOS developer.

What is the advantage of push?
------------------------------

Without Push, your iPhone will Fetch email periodically. This means that mail is delayed and that your device will use precious power to connect to a remote IMAP server.

Using native Push messages means that your phone will receive new email notifications over a highly optimized connection to iCloud that it already has for other purposes.

High Level Overview
-------------------

There are two parts to enabling iOS Push Email. You will need both parts for this to work.

First you need to install the Dovecot plugins from the [dovecot-xaps-plugin](https://github.com/st3fan/dovecot-xaps-plugin) project. How do to that is documented in the README file in that project. The Dovecot plugin adds support for the `XAPPLEPUSHSERVICE` IMAP extension that will let iOS devices register themselves to receive native push notifications for new email arrival.

(Apple did not document this feature, but it did publish the source code for all their Dovecot patches on the [Apple Open Source project site](http://www.opensource.apple.com/source/dovecot/dovecot-293/), which include this feature. Although I was not able to follow a specification, I was able to read their open source project and do a clean implementation with all original code.)

Second, you need to install a daemon process (contained in this project) that will be responsible for receiving new email notifications from the Dovecot Local Delivery Agent and transforming those into native Apple Push Notifications.

Installation
============

Prerequisites
-------------

You are going to need the following things to get this going:

* Some patience and willingness to experiment - Although I run this project in production, it is still a very early version and it may contain bugs.
* Because you will need a certificate to talk to the Apple Push Notifications Service, you can only run this software if you are migrating away from an existing OS X Server setup where you had Push Email enabled.
* Dovecot > 2.2.11 (which fixed an EPIPE Bug) AND Dovecot < 2.3.0 (which has several changes in the mailbox_vfuncs signatures) 

Exporting and converting the certificate
----------------------------------------

First you have to export the certificate that is stored on your OS X
Server. Do this by opening Keychain.app and select the System keychain and the Certificates category. Locate the right certificate by expanding those whose name start with *APSP:* and then look for the certificate with a private key that is named `com.apple.servermgrd.apns.mail`.

Now export that certficate by selecting it and then choose *Export Items* from the *File* menu. You want to store the certificate as PushEmail on your Desktop as a *Personal Information Exchange (.p12)* file. You will be asked to secure this exported certificate with a password. This is a new password and not your login password.

Then, start *Terminal.app* and execute the following commands:

```
cd ~/Desktop
openssl pkcs12 -in PushEmail.p12 -nocerts -nodes -out key.pem
openssl pkcs12 -in PushEmail.p12 -clcerts -nokeys -out certificate.pem
```

You will be asked for a password, which should be the same password that you entered when you exported the certificate.

You can test if the certificate and key are correct by making a connection to the apple push notifications gateway:

```
openssl s_client -connect gateway.push.apple.com:2195 -cert certificate.pem -key key.pem
```

The connection may close but check if you see something like `Verify return code: 0 (ok)` appear.

If the connection fails and outputs `Verify return code: 20 (unable to get local issuer certificate)` the chain of trust might be broken. Download the root certificate entrust_2048_ca.cer from [Entrust] (https://www.entrust.net/downloads/root_index.cfm?) and issue the command appending -CAfile:

```
openssl s_client -connect gateway.push.apple.com:2195 -cert certificate.pem -key key.pem -CAfile entrust_2048_ca.cer
```

> TODO: Does this mean we also need to pass the CA file to the `xapsd` process?

You now have your exported certificate and private key stored in two separate PEM encoded files that can be used by the xapsd daemon.

Copy these two files to your Dovecot server.

> Note that the APNS certificates expire 1 year after they were originally issued by Apple, so they will need to be renewed or regenerated through the OS X Server application each year. Expiration information for these certificates can be found at the [Apple Push Certificates Portal](https://identity.apple.com/pushcert/).

Compiling and Installing the Daemon
-----------------------------------

The daemon is written in Go. The easiest way to build it is with go itself.

```
git clone https://github.com/st3fan/dovecot-xaps-daemon.git
cd dovecot-xaps-daemon
go build -o xapsd
```

Running the Daemon
------------------

Because this code is work in progress, it currently is not packaged properly as a good behaving background process. I recommend following the instructions below in a `screen` or `tmux` session so that it is easy to keep the daemon running. The next release will have better support for running this as a background service.

You can run the daemon as follows:

```
bin/xapsd -key=$HOME/key.pem -certificate=$HOME/certificate.pem \
  -database=$HOME/xapsd.json -socket=/tmp/xapsd.sock
```

This assumes that you have the exported `certificate.pem` and `key.pem` files in your home directory. The database file will be created by the daemon. It will contain the mappings between the IMAP users, their mail accounts and the iOS devices. It is a simple JSON file so you can look at it manually by opening it in a text editor.

The daemon is verbose and should print out a bunch of informational messages. If you see errors, please [file a bug](https://github.com/st3fan/dovecot-xaps-daemon/issues/new).


Setting up Devices
------------------

Your iOS devices will discover that the server supports Push automatically the first time they connect. To force them to reconnect you can reboot the iOS device or turn Airport Mode on and off with a little delay in between.

If you go to your Email settings, you should see that the account has switched to Push.
