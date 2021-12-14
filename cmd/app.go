// Package cmd provides the ntfybot CLI application
package cmd

import (
	"errors"
	"fmt"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
	"log"
	"ntfy-bot/bot"
	"ntfy-bot/config"
	"ntfy-bot/util"
	"os"
	"os/signal"
	"syscall"
)

// New creates a new CLI application
func New() *cli.App {
	flags := []cli.Flag{
		&cli.StringFlag{Name: "config", Aliases: []string{"c"}, EnvVars: []string{"NTFY_BOT_CONFIG_FILE"}, Value: "/etc/ntfy/bot.yml", DefaultText: "/etc/ntfy/bot.yml", Usage: "config file"},
		&cli.BoolFlag{Name: "debug", EnvVars: []string{"NTFY_BOT_DEBUG"}, Value: false, Usage: "enable debugging output"},
		altsrc.NewStringFlag(&cli.StringFlag{Name: "bot-token", Aliases: []string{"t"}, EnvVars: []string{"NTFY_BOT_TOKEN"}, DefaultText: "none", Usage: "bot token"}),
	}
	return &cli.App{
		Name:                   "ntfybot",
		Usage:                  "Slack/Discord bot for sending and receiving messages to/from ntfy",
		UsageText:              "ntfybot [OPTION..]",
		HideHelp:               true,
		HideVersion:            true,
		EnableBashCompletion:   true,
		UseShortOptionHandling: true,
		Reader:                 os.Stdin,
		Writer:                 os.Stdout,
		ErrWriter:              os.Stderr,
		Action:                 execRun,
		Before:                 initConfigFileInputSource("config", flags),
		Flags:                  flags,
	}
}

func execRun(c *cli.Context) error {
	// Read all the options
	token := c.String("bot-token")
	debug := c.Bool("debug")

	// Validate options
	if token == "" || token == "MUST_BE_SET" {
		return errors.New("missing bot token, pass --bot-token, set NTFY_BOT_TOKEN env variable or bot-token config option")
	}

	// Create main bot
	conf := config.New(token)
	conf.Debug = debug
	robot, err := bot.New(conf)
	if err != nil {
		return err
	}

	// Set up signal handling
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs // Doesn't matter which
		log.Printf("Signal received. Closing all active sessions.")
		robot.Stop()
	}()

	// Run main bot, can be killed by signal
	if err := robot.Run(); err != nil {
		return err
	}

	log.Printf("Exiting.")
	return nil
}

// initConfigFileInputSource is like altsrc.InitInputSourceWithContext and altsrc.NewYamlSourceFromFlagFunc, but checks
// if the config flag is exists and only loads it if it does. If the flag is set and the file exists, it fails.
func initConfigFileInputSource(configFlag string, flags []cli.Flag) cli.BeforeFunc {
	return func(context *cli.Context) error {
		configFile := context.String(configFlag)
		if context.IsSet(configFlag) && !util.FileExists(configFile) {
			return fmt.Errorf("config file %s does not exist", configFile)
		} else if !context.IsSet(configFlag) && !util.FileExists(configFile) {
			return nil
		}
		inputSource, err := altsrc.NewYamlSourceFromFile(configFile)
		if err != nil {
			return err
		}
		return altsrc.ApplyInputSourceValues(context, inputSource, flags)
	}
}
