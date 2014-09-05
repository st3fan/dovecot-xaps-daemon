
iOS Push Email for Dovecot
==========================

What is this?
-------------

This project, together with the [dovecot-xaps-plugin](https://github.com/st3fan/dovecot-xaps-plugin) project, will enable push email for iOS devices that talk to your Dovecot 2.0.x IMAP server. This is specially useful for people who are migrating away from running email services on OS X Server and want to keep the Push Email ability.

> Please note that it is not possible to use this project without legally owning a copy of OS X Server. Please do not pirate OS X Server. Instead you can find it on the [Mac App Store](https://itunes.apple.com/ca/app/os-x-server/id714547929?mt=12).

High Level Overview
-------------------

There are two parts to enabling iOS Push Email. You will need both parts for this to work.

First you need to install the Dovecot plugins from the [dovecot-xaps-plugin](https://github.com/st3fan/dovecot-xaps-plugin) project. How do to that is documented in the README file in that project. The Dovecot plugin adds support for the `XAPPLEPUSHSERVICE` IMAP extension that will let iOS devices register themselves to receive native push notifications for new email arrival.

(Apple did not document this feature, but it did publish the source code for all their Dovecot patches on the [Apple Open Source project site](http://www.opensource.apple.com/source/dovecot/dovecot-293/), which include this feature. So although I was not able to follow a specification, I was able to read their open source project and do a clean implementation with all original code.)

Second, you need to install a daemon process (contained in this project) that will be responsible for receiving new email notifications from the Dovecot Local Delivery Agent and transforming those into native Apple Push Notifications.

Installation
============

Prerequisites
-------------

You are going to need the following things to get this going:

* Some patience and willingness to experiment - Although I run this project in production, it is still a very early version and it may contain bugs.
* Because you will need a certificate to talk to the Apple Push Notifications Service, you can only run this software if you are migrating away from an existing OS X Server setup where you had Push Email enabled.
* This software has only been tested on Ubuntu 12.04.5 with Dovecot 2.0.19. So ideally you have a mail server with the same specifications, or something very similar.

Exporting and converting the certificate
----------------------------------------

First you have to export the certificate that is stored on your OS X
Server. Do this by opening Keychain.app and select the System keychain and the Certificates category. locate the certificate by expanding the ones that start with *APSP:* and look for a private key named `com.apple.servermgrd.apns.mail`.

Now export the certficate by selecting it and then choose *Export Items* from the *File* menu. You want to store the certificate as PushEmail on your Desktop as a *.p12* file.

Then, open a terminal window and execute the following commands:

```
cd ~/Desktop
openssl pkcs12 -in PushEmail.p12 -nocerts -nodes -out key.pem
openssl pkcs12 -in PushEMail.p12 -clcerts -nokeys -out certificate.pem
```

You will be asked for a password, which is the same password that you entered when you exported the certificate.

You can test if the certificate and key are correct by making a connection to the apple push notifications gateway:

```
openssl s_client -connect gateway.push.apple.com:2195 -cert certificate.pem -key key.pem
```

The connection may close but check if you see something like `Verify return code: 0 (ok)` appear.

You now have your exported certificate and private key stored in two separate PEM encoded files that can be used by the xapsd daemon.

Copy these two files to your Dovecot server.

By default the `xapsd` daemon will look for these files in the following location:

 * `/etc/xapsd/certificate.pem`
 * `/etc/xapsd/key.pem`

Installing and Running the Daemon
---------------------------------

Because this code is work in progress, it currently is not packaged properly as a good behaving background process. The following instructions will work fine but will likely change later on.

First, install the following Ubuntu 12.04.5 packages, or equivalent for your operating system:

```
sudo apt-get install git python-twisted
```

Then clone this project:

```
git clone https://github.com/st3fan/dovecot-xaps-daemon.git
```

You can now run the daemon as follows:

```
cd dovecot-xaps-daemon
./xapsd --socket=/tmp/xapsd.sock --database=$HOME/xapsd.json --certificate=$HOME/certificate.pem --key=$HOME/key.pem
```

This assumes that you have the exported `certificate.pem` and `key.pem` files in your home directory.

The daemon is verbose and should print out a bunch of informational messages. If you see errors, please file a bug.
