//
// The MIT License (MIT)
//
// Copyright (c) 2015 Stefan Arentz <stefan@arentz.ca>
// Copyright (c) 2017 Frederik Schwan <frederik dot schwan at linux dot com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
//

package main

import (
	"flag"
	"github.com/freswa/dovecot-xaps-daemon/aps"
	"github.com/freswa/dovecot-xaps-daemon/config"
	"github.com/freswa/dovecot-xaps-daemon/database"
	"github.com/freswa/dovecot-xaps-daemon/logger"
	"github.com/freswa/dovecot-xaps-daemon/socket"
	log "github.com/sirupsen/logrus"
)

const Version = "1.1"

var configPath = flag.String("configName", "", `Add an additional path to lookup the config file in`)
var configName = flag.String("configPath", "", `Set a different configName (without extension) than the default "xapsd"`)


func main() {
	config.ParseConfig(*configName, *configPath)
	config := config.GetOptions()
	flag.Parse()
	logger.ParseLoglevel(config.LogLevel)

	log.Debugln("Opening databasefile at", config.DatabaseFile)
	db, err := database.NewDatabase(config.DatabaseFile)
	if err != nil {
		log.Fatal("Cannot open databasefile: ", config.DatabaseFile)
	}
	topic := aps.NewApns(config.CertFile, config.KeyFile, config.CheckInterval, config.Delay, db)

	log.Printf("Starting to listen on %s", config.SocketPath)
	socket.NewSocket(config.SocketPath, db, topic)
}
