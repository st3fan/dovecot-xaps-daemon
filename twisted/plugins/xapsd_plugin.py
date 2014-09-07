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


from zope.interface import implements

from twisted.application.internet import SSLClient, UNIXServer
from twisted.application.service import IServiceMaker, MultiService
from twisted.internet.ssl import DefaultOpenSSLContextFactory
from twisted.plugin import IPlugin
from twisted.python import usage

from xaps import XAPSFactory, RegistrationDatabase
from apns import APNSClientFactory


class Options(usage.Options):

    optParameters = [
        ["socket", "", "/var/run/dovecot/xapsd", "Path to the socket"],
        ["database", "", "/var/lib/dovecot/xapsd.json", "Path to the registration database"],
        ["key", "", "/etc/xapsd/key.pem", "Path to the private key PEM file"],
        ["certificate", "", "/etc/apsd/certificate.pem", "Path to the certificate PEM file"],
        ["apns-host", "", "gateway.push.apple.com", "APNS Hostname"],
        ["apns-port", "", "2195", "APNS Port"],
    ]


def parseTopicFromCertificate(certificatePath):
    with open(certificatePath) as fp:
        certificateData = fp.read()
        certificate = crypto.load_certificate(crypto.FILETYPE_PEM, certificateData)
        subjectComponents = dict(certificate.get_subject().get_components())
        return subjectComponents.get('UID')


class ServiceMaker(object):

    implements(IServiceMaker, IPlugin)

    tapname = "xapsd"
    description = "Backend for iOS Push Notifications"
    options = Options

    def makeService(self, options):

        service = MultiService()

        contextFactory = DefaultOpenSSLContextFactory(options["key"], options["certificate"])
        apnsFactory = APNSClientFactory()
        apnsService = SSLClient(options["apns-host"], int(options["apns-port"]), apnsFactory, contextFactory)
        apnsService.setServiceParent(service)

        database = RegistrationDatabase(options["database"])
        topicName = parseTopicFromCertificate(options["certificate"])

        xapsService = UNIXServer(options["socket"], XAPSFactory(database, topicName, apnsFactory))
        xapsService.setServiceParent(service)

        return service


serviceMaker = ServiceMaker()
