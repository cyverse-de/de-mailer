package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DavidGamba/go-getoptions"
	"github.com/cyverse-de/configurate"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var log = logrus.WithFields(logrus.Fields{"service": "de-mailer"})

type commandLineOptionValues struct {
	Config string
}

type DESettings struct {
	base        string
	data        string
	analyses    string
	teams       string
	tools       string
	collections string
	apps        string
	admin       string
	doi         string
	vice        string
}

// getErrorResponseCode returns the response code to use for the given error.
func getErrorResponseCode(err error) int {
	if httpError, ok := errors.AsType[*HTTPError](err); ok {
		return httpError.Code()
	}
	return http.StatusInternalServerError
}

// parseCommandLine parses the command line and returns an options structure containing command-line options and
// parameters.
func parseCommandLine() *commandLineOptionValues {
	optionValues := &commandLineOptionValues{}
	opt := getoptions.New()

	// Default option values.
	defaultConfigPath := "/etc/iplant/de/emailservice.yml"

	// Define the command-line options.
	opt.Bool("help", false, opt.Alias("h", "?"))
	opt.StringVar(&optionValues.Config, "config", defaultConfigPath,
		opt.Alias("c"),
		opt.Description("the path to the configuration file"))

	// Parse the command line, handling requests for help and usage errors.
	_, err := opt.Parse(os.Args[1:])
	if opt.Called("help") {
		fmt.Fprint(os.Stderr, opt.Help())
		os.Exit(0)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n\n", err)
		fmt.Fprint(os.Stderr, opt.Help(getoptions.HelpSynopsis))
		os.Exit(1)
	}

	return optionValues
}

// validateConfig fails fast on settings that would otherwise cause silent misbehavior at runtime.
func validateConfig(config *viper.Viper) error {
	requiredKeys := []string{
		"email.smtpHost",
		"email.fromAddress",
		"de.base",
		"amqp.uri",
		"amqp.exchange.name",
		"amqp.exchange.type",
	}
	for _, key := range requiredKeys {
		if config.GetString(key) == "" {
			return fmt.Errorf("configuration setting %s must not be empty; check the mounted emailservice.yml", key)
		}
	}
	return nil
}

func main() {
	// Parse the command line.
	optionValues := parseCommandLine()

	// Load and validate the configuration. Init (not InitDefaults) is deliberate: the
	// library defaults include a non-empty amqp.uri, which would make the required-key
	// validation below unable to catch a missing amqp section.
	config, err := configurate.Init(optionValues.Config)
	if err != nil {
		log.Fatal(err)
	}
	if err := validateConfig(config); err != nil {
		log.Fatal(err)
	}

	emailClient := NewEmailClient(config.GetString("email.smtpHost"), config.GetString("email.fromAddress"))

	deSettings := &DESettings{
		base:        config.GetString("de.base"),
		data:        config.GetString("de.data"),
		analyses:    config.GetString("de.analyses"),
		teams:       config.GetString("de.teams"),
		tools:       config.GetString("de.tools"),
		collections: config.GetString("de.collections"),
		apps:        config.GetString("de.apps"),
		admin:       config.GetString("de.admin"),
		doi:         config.GetString("de.doi"),
		vice:        config.GetString("de.vice"),
	}

	processor := NewEmailProcessor(emailClient, deSettings, config.GetString("email.fromAddress"))
	api := NewAPI(processor)

	amqpSettings := &AMQPSettings{
		URI:          config.GetString("amqp.uri"),
		ExchangeName: config.GetString("amqp.exchange.name"),
		ExchangeType: config.GetString("amqp.exchange.type"),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The consumer starts in the background so a broker outage can't keep the HTTP
	// transport from coming up; startAMQPConsumer retries until the broker is reachable.
	listenerReady := make(chan *AMQPListener, 1)
	go func() {
		if listener := startAMQPConsumer(ctx, processor, amqpSettings); listener != nil {
			listenerReady <- listener
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(api.EmailRequestHandler))
	server := &http.Server{Addr: ":8080", Handler: mux}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()
	log.Infof("listening on %s", server.Addr)

	select {
	case err := <-serverErr:
		log.Fatal(err)
	case <-ctx.Done():
	}

	// Ordered shutdown: let in-flight AMQP handlers finish and ack (so already-sent
	// emails aren't requeued and re-sent), close the connection, then drain HTTP.
	var listener *AMQPListener
	select {
	case listener = <-listenerReady:
	default:
	}
	if listener != nil {
		listener.Drain(20 * time.Second)
		listener.Close()
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Errorf("HTTP server shutdown: %s", err)
	}
}
