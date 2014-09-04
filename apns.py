#!/usr/bin/python


import json
import sys
import struct

from twisted.internet.protocol import Protocol, ReconnectingClientFactory
from twisted.internet.ssl import DefaultOpenSSLContextFactory
from twisted.internet import reactor
from twisted.internet.task import LoopingCall
from twisted.python import log


class APNSNotification:

    def __init__(self, deviceToken, payload, priority=None):
        self.deviceToken = deviceToken
        self.payload = payload
        self.priority = priority

    def _createDeviceTokenItem(self, deviceToken):
        return struct.pack(">BH", 1, 32) +deviceToken.decode('hex')

    def _createPayloadItem(self, payload):
        json_payload = json.dumps(payload, separators=(',', ':'))
        return struct.pack(">BH", 2, len(json_payload)) + json_payload

    def _createPriorityItem(self, priority):
        return struct.pack(">BHB", 5, 1, priority)

    def _createFrame(self, items):
        item_data = "".join(items)
        return struct.pack(">BI", 2, len(item_data)) + item_data

    def serialize(self):
        return self._createFrame([self._createDeviceTokenItem(self.deviceToken), self._createPayloadItem(self.payload)])


class APNSProtocol(Protocol):

    def connectionMade(self):
        log.msg("APNSProtocol.connectionMade")
        self.factory.clientConnectionMade(self)

    def connectionLost(self, reason):
        log.msg("APNSProtocol.connectionLost")
        self.factory.clientConnectionMade(self)

    def dataReceived(self, data):
        log.msg("APNSProtocol.dataReceived: " + data.encode("hex"))

    def sendNotification(self, notification):
        log.msg("APNSProtocol.sendNotification")
        frame = notification.serialize()
        self.transport.write(frame)

    def sendNotifications(self, notifications):
        log.msg("APNSProtocol.sendNotifications (%d)" % len(notifications))
        for notification in notifications:
            frame = notification.serialize()
            self.transport.write(frame)


class APNSClientFactory(ReconnectingClientFactory):

    def __init__(self):
        log.msg("APNSClientFactory.__init__")
        self.looper = LoopingCall(self.sendOutstandingNotifications)
        self.outstandingNotifications = []

    def startedConnecting(self, connector):
        log.msg("APNSClientFactory.startedConnecting")

    def buildProtocol(self, address):
        log.msg("APNSClientFactory.buildProtocol")
        protocol = APNSProtocol()
        protocol.factory = self
        return protocol

    def clientConnectionLost(self, connector, reason):
        log.msg("APNSClientFactory.clientConnectionLost")
        ReconnectingClientFactory.clientConnectionLost(self, connector, reason)

    def clientConnectionFailed(self, connector, reason):
        log.msg("APNSClientFactory.clientConnectionFailed")
        ReconnectingClientFactory.clientConnectionFailed(self, connector, reason)

    def clientConnectionMade(self, client):
        log.msg("APNSClientFactory.clientConnectionMade")
        self.client = client
        self.looper.start(2.5)

    def clientConnectionLost(self, client):
        log.msg("APNSClientConnectionLost")
        self.client = None
        self.looper.stop()

    def sendOutstandingNotifications(self):
        #log.msg("APNSClientFactory.sendOutstandingNotifications")
        if len(self.outstandingNotifications) > 0:
            notifications = self.outstandingNotifications[:25]
            self.outstandingNotifications[:25] = []
            self.client.sendNotifications(notifications)

    # Public API

    def queueNotification(self, notification):
        log.msg("APNSClientFactory.queueNotification")
        self.outstandingNotifications.append(notification)


class APNSService:

    def __init__(self, certificatePath, keyPath, host="gateway.push.apple.com", port=2195):
        log.msg("APNSService.__init__")
        self.certificatePath = certificatePath
        self.keyPath = keyPath
        self.host = host
        self.port = port
        self.clientFactory = APNSClientFactory()

    def queueNotification(self, notification):
        log.msg("APNSService.queueNotification")
        self.clientFactory.queueNotification(notification)

    def start(self):
        log.msg("APNSService.start")
        return reactor.connectSSL(self.host, self.port, self.clientFactory,
                                  DefaultOpenSSLContextFactory(self.keyPath, self.certificatePath))


if __name__ == "__main__":

    log.startLogging(sys.stdout)

    service = APNSService("/home/stefan/certificate.pem", "/home/stefan/key.pem")
    service.start()

    def queueTestNotification():
        notification = APNSNotification("361E1CF19D03E6A3380AB34B83399F1123FF523F9AC7AB2F3ADA531DDD9A96C1",
                                        {'aps':{'account-id': '1B737D45-5B98-48B0-BD2F-571343D03F85'}})
        service.queueNotification(notification)

    reactor.callLater(3, queueTestNotification)

    reactor.run()
