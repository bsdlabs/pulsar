/*
 * SPDX-License-Identifier: BSD-2-Clause
 *
 * Copyright (c) 2021, Lewis Cook <lcook@FreeBSD.org>
 * All rights reserved.
 */
package main

import (
	"flag"
	"fmt"
	"net/http"

	nested "github.com/antonfisher/nested-logrus-formatter"
	command "github.com/bsdlabs/pulseline/internal/command/bug"
	"github.com/bsdlabs/pulseline/internal/config"
	"github.com/bsdlabs/pulseline/internal/hook/git"
	ver "github.com/bsdlabs/pulseline/internal/version"
	"github.com/bwmarrin/discordgo"
	"github.com/lcook/hookrelay"
	log "github.com/sirupsen/logrus"
)

type (
	Handler []hookrelay.Hook
)

func main() {
	var (
		verboseLevel int
		cfgFile      string
		version      bool
		color        bool
	)

	flag.IntVar(&verboseLevel, "V", 1, "Log verbosity level (1-3)")
	flag.StringVar(&cfgFile, "c", "config.yaml", "YAML configuration file path")
	flag.BoolVar(&version, "v", false, "Display pulseline version")
	flag.BoolVar(&color, "d", false, "Disable color output in logs")
	flag.Parse()

	log.SetFormatter(&nested.Formatter{
		ShowFullLevel:    true,
		NoUppercaseLevel: true,
		TrimMessages:     true,
		TimestampFormat:  "[02/Jan/2006:15:04:05]",
		NoFieldsColors:   true,
		NoColors:         color,
	})

	if version {
		fmt.Println(ver.Build)
		return
	}
	/*
	 * Clamp the verbosity with an lower bound of 1 and
	 * upper bound of 3 (1-3).
	 */
	verboseClamp := func(level, lower, upper int) int {
		if level < lower {
			return lower
		}

		if level > upper {
			return upper
		}

		return level
	}(verboseLevel, 1, 3)

	switch verboseClamp {
	case 1:
		log.SetLevel(log.InfoLevel)
	case 2:
		log.SetLevel(log.DebugLevel)
	case 3:
		log.SetLevel(log.TraceLevel)
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("could not open configuration file")
	}

	log.Infof("loaded configuration settings (%s)", cfgFile)
	log.Printf("init discord ...")

	session, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("could not create discord session")
	}

	err = session.Open()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("could not open discord connection")
	}

	log.WithFields(log.Fields{
		"id":   session.State.User.ID,
		"user": session.State.User.Username,
	}).Info("discord session started")

	session.AddHandler(command.BugHandler)
	session.Identify.Intents = discordgo.IntentsGuildMessages

	_ = session.UpdateGameStatus(0, ver.Build)

	log.Printf("init pulseline-%s ...", ver.Build)

	srv, err := hookrelay.InitMux(session, Handler{
		&git.Pulse{
			Option: (hookrelay.DefaultOptions),
		},
	}, cfgFile, cfg.Port)

	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("could not start pulseline server")
	}

	log.WithFields(log.Fields{
		"port": cfg.Port,
	}).Info("pulseline server started")

	if err := srv.ListenAndServe(); err != nil &&
		err != http.ErrServerClosed {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("could not listen on port")
	}
	//nolint
	session.Close()
}
