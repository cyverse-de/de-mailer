package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/DavidGamba/go-getoptions"
	"github.com/cyverse-de/configurate"
	"github.com/cyverse-de/go-mod/otelutils"
	"github.com/sirupsen/logrus"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var log = logrus.WithFields(logrus.Fields{"service": "de-mailer"})

const otelName = "github.com/cyverse-de/de-mailer"
const serviceName = "de-mailer"

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
	if httpError, ok := err.(*HTTPError); ok {
		return httpError.Code()
	}
	return http.StatusInternalServerError
}

// copied from https://github.com/cyverse-de/email-requests/blob/master/main.go
// parseCommandLine parses the command line and returns an options structure containing command-line options and
// parameters.
// commandLineOptionValues represents the values of the options that were passed on the command line when this
// service was invoked.
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

// parseRequestBody will parse the post body. The body is the notification message
func parseRequestBody(r *http.Request) (EmailRequest, map[string](any), error) {
	var emailReq EmailRequest
	// unmarshall payload to map with interface{}
	payloadMap := make(map[string](any))

	body, err := io.ReadAll(r.Body)
	if err != nil {
		httpError := NewHTTPError(http.StatusInternalServerError, "failed to read request body: %s", err)
		log.Error(httpError)
		return emailReq, payloadMap, httpError
	}
	fmt.Println(string(body))

	err = json.Unmarshal(body, &emailReq)

	if err != nil {
		httpError := NewHTTPError(http.StatusBadRequest, "failed to parse request body: %s", err)
		log.Error(httpError)
		return emailReq, payloadMap, httpError
	} else {
		err := json.Unmarshal(emailReq.Values, &payloadMap)
		if err != nil {
			httpError := NewHTTPError(http.StatusBadRequest, "failed to parse template values: %s", err.Error())
			log.Error(httpError)
			return emailReq, payloadMap, httpError
		}
		return emailReq, payloadMap, err
	}
}

func main() {
	var tracerCtx, cancel = context.WithCancel(context.Background())
	defer cancel()
	shutdown := otelutils.TracerProviderFromEnv(tracerCtx, serviceName, func(e error) { log.Fatal(e) })
	defer shutdown()

	// Parse the command line.
	optionValues := parseCommandLine()

	// Load the configuration.
	config, err := configurate.InitDefaults(optionValues.Config, configurate.JobServicesDefaults)
	if err != nil {
		log.Fatal(err)
	}
	//test config
	log.Info(config.GetString("de.base"))

	emailClient := NewEmailClient(config.GetString("email.smtpHost"), config.GetString("email.fromAddress"))

	deSettings := DESettings{
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

	api := NewAPI(emailClient, &deSettings)
	http.Handle("/", otelhttp.NewHandler(http.HandlerFunc(api.EmailRequestHandler), "/"))
	log.Fatal(http.ListenAndServe(":8080", nil))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
