#!/usr/bin/env python

#
# The MIT License (MIT)
#
# Copyright (c) 2014 Stefan Arentz <stefan@arentz.ca>
#
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in
# all copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
# THE SOFTWARE.
#


import argparse
import collections
import hashlib
import json
import os
import sys
import struct
import OpenSSL.crypto as crypto

from twisted.enterprise.adbapi import ConnectionPool
from twisted.internet import reactor
from twisted.internet.endpoints import SSL4ClientEndpoint
from twisted.internet.protocol import ServerFactory, Protocol, Factory
from twisted.internet.ssl import DefaultOpenSSLContextFactory
from twisted.protocols.basic import LineOnlyReceiver
from twisted.python import log
from twisted.python.filepath import FilePath
from twisted.web.xmlrpc import Proxy

from apns import APNSNotification, APNSService



def connectProtocol(endpoint, protocol):
    class OneShotFactory(Factory):
        def buildProtocol(self, addr):
            return protocol
    return endpoint.connect(OneShotFactory())


#
# Command is simple named tuple in which we store a parsed commands
# from the Dovecot plugins.
#

Command = collections.namedtuple('Command', ['name', 'args'])

def unescape_value(v):
    if v.startswith('"') and v.endswith('"'):
        return v[1:-1].decode("string_escape")

def parse_list(value):
    return [unescape_value(v) for v in value[1:-1].split(",")]

def parse_command(line):
    args = {}
    command_name, rest = line.split(" ", 1)
    for pair in rest.split("\t"):
        name, value = pair.split('=', 1)
        if value.startswith("(") and value.endswith(")"):
            args[name] = parse_list(value)
        else:
            args[name] = unescape_value(value)
    return Command(command_name, args)


#
# Simple in memory registration database. This works fine for me
# because I have a small number of users, accounts and devices. For
# anything larger scale this probably needs to be improved.
#

class RegistrationDatabase:

    def __init__(self, path):
        self.path = path
        self.registrations = {}
        if os.path.exists(self.path):
            with open(self.path) as fp:
                self.registrations = json.load(fp)

    def addRegistration(self, username, accountId, deviceToken, mailboxes):
        if username not in self.registrations:
            self.registrations[username] = {"accounts": {}}
        if accountId not in self.registrations[username]["accounts"]:
            self.registrations[username]["accounts"][accountId] = {"devices": {}}
        self.registrations[username]["accounts"][accountId]["devices"][deviceToken] = {"mailboxes":mailboxes}
        with open(self.path, "w") as fp:
            json.dump(self.registrations, fp, indent=4)

    def findRegistrations(self, username, mailbox):
        if username in self.registrations:
            for accountId,account in self.registrations[username]["accounts"].items():
                for deviceToken, device in account["devices"].items():
                    if mailbox in device["mailboxes"]:
                        yield (deviceToken, accountId)


#
# This is the protocol that we speak from our Dovecot plugins. It
# listens to two commands, REGISTER and NOTIFY. Both commands take a
# variable number of key value pairs that looks like this:
#
#  COMMAND arg1="val1" arg2="val2"
#

class XAPSProtocol(LineOnlyReceiver):

    def __init__(self, database, topicName, notificationService):
        self.database = database
        self.topicName = topicName
        self.notificationService = notificationService
        self.commandHandlers = dict(REGISTER=self.handleRegister, NOTIFY=self.handleNotify)

    #
    # Handle the REGISTER command. It looks as follows:
    #
    #  REGISTER aps-account-id="AAA" aps-device-token="BBB"
    #     aps-subtopic="com.apple.mobilemail"
    #     dovecot-username="stefan"
    #     dovecot-mailboxes=("Inbox","Notes")
    #
    # We simply store the whole thing in the database?
    #
    # The command returns the aps-topic, which is the common name of
    # the certificate issued by OS X Server for email push
    # notifications.
    #

    def handleRegister(self, cmd):
        log.msg("handleRegister: " + str(cmd))
        if cmd.args['aps-subtopic'] != "com.apple.mobilemail":
            return ("ERROR", "Unknown aps-subtopic")
        self.database.addRegistration(cmd.args['dovecot-username'], cmd.args['aps-account-id'],
                                      cmd.args['aps-device-token'], cmd.args['dovecot-mailboxes'])
        return ("OK", self.topicName)
    #
    # Handle the NOTIFY command. It looks as follows:
    #
    #  NOTIFY dovecot-username="stefan" dovecot-mailbox="Inbox"
    #
    # See if the the username has devices registered. If it has, loop
    # over them to find the ones that are interested in the named
    # mailbox and send those a push notificiation.
    #
    # The push notification looks like this:
    #
    #  { "aps": { "account-id": aps-account-id } }
    #

    def handleNotify(self, command):
        log.msg("handleNotify: " + str(command))
        for deviceToken, accountId in self.database.findRegistrations(command.args['dovecot-username'], command.args['dovecot-mailbox']):
            self.notificationService.queueNotification(APNSNotification(deviceToken, {"aps":{"account-id":accountId}}))
        return ("OK", "")

    #
    # Handle unknown commands. Simply return an error message.
    #

    def handleUnknownCommand(self, command):
        return ("ERROR", "Unknown command")

    #
    # Process an incoming line. Parse the command, dispatch to the
    # right handler.
    #

    def lineReceived(self, line):
        log.msg("lineReceived: " + line)
        command = parse_command(line)
        log.msg(str(command))
        if not command:
            self.sendLine("ERROR Cannot parse command")
        else:
            status, message = self.commandHandlers.get(command.name, self.handleUnknownCommand)(command)
            self.sendLine(status + " " + message)


class XAPSFactory(ServerFactory):

    def __init__(self, database, topicName, notificationService):
        self.database = database
        self.topicName = topicName
        self.notificationService = notificationService

    def buildProtocol(self, addr):
        return XAPSProtocol(self.database, self.topicName, self.notificationService)


#
# Main program starts here
#

def parseTopicFromCertificate(certificatePath):
    with open(certificatePath) as fp:
        certificateData = fp.read()
        certificate = crypto.load_certificate(crypto.FILETYPE_PEM, certificateData)
        subjectComponents = dict(certificate.get_subject().get_components())
        return subjectComponents.get('UID')


def main(socket_path, database_path, certificate_path, key_path):

    log.startLogging(sys.stdout)

    # Database
    database = RegistrationDatabase(database_path)

    # Parse the certificate to obtain the topic name
    topic = parseTopicFromCertificate(certificate_path)
    log.msg("Topic from certificate is " + topic)

    # Check if our socket already exists
    address = FilePath(socket_path)
    if address.exists():
        raise SystemExit("Cannot listen on an existing path")

    notificationService = APNSService(certificate_path, key_path)
    notificationService.start()

    # Start listening on the socket
    factory = XAPSFactory(database, topic, notificationService)
    port = reactor.listenUNIX(address.path, factory)
    reactor.run()


if __name__ == "__main__":

    parser = argparse.ArgumentParser(prog="xapsd")
    parser.add_argument("--socket", default="/var/run/dovecot/xapsd.sock")
    parser.add_argument("--database", default="/var/lib/dovecot/xapsd.json")
    parser.add_argument("--certificate", default="/etc/xapsd/certificate.pem")
    parser.add_argument("--key", default="/etc/xapsd/key.pem")

    args = parser.parse_args()

    main(args.socket, args.database, args.certificate, args.key)
