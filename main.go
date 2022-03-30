package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/DavidGamba/go-getoptions"
	"github.com/cyverse-de/configurate"
	"github.com/sirupsen/logrus"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
)

var log = logrus.WithFields(logrus.Fields{"service": "de-mailer"})

const otelName = "github.com/cyverse-de/de-mailer"

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

//copied from https://github.com/cyverse-de/email-requests/blob/master/main.go
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
func parseRequestBody(r *http.Request) (EmailRequest, map[string](interface{}), error) {
	var emailReq EmailRequest
	// unmarshall payload to map with interface{}
	payloadMap := make(map[string](interface{}))

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		msg := fmt.Sprintf("failed to read request body: %s", err.Error())
		log.Error(msg)
		return emailReq, payloadMap, NewHTTPError(http.StatusInternalServerError, msg)
	}
	fmt.Println(string(body))

	err = json.Unmarshal(body, &emailReq)

	if err != nil {
		msg := fmt.Sprintf("failed to parse request body: %s", err.Error())
		log.Error(msg)
		return emailReq, payloadMap, NewHTTPError(http.StatusBadRequest, msg)
	} else {
		err := json.Unmarshal(emailReq.Values, &payloadMap)
		if err != nil {
			msg := fmt.Sprintf("failed to parse template values: %s", err.Error())
			log.Error(msg)
			return emailReq, payloadMap, NewHTTPError(http.StatusBadRequest, msg)
		}
		return emailReq, payloadMap, err
	}
}

func jaegerTracerProvider(url string) (*tracesdk.TracerProvider, error) {
	// Create the Jaeger exporter
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		return nil, err
	}

	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exp),
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("de-mailer"),
		)),
	)

	return tp, nil
}

func main() {
	var tracerProvider *tracesdk.TracerProvider
	otelTracesExporter := os.Getenv("OTEL_TRACES_EXPORTER")
	if otelTracesExporter == "jaeger" {
		jaegerEndpoint := os.Getenv("OTEL_EXPORTER_JAEGER_ENDPOINT")
		if jaegerEndpoint == "" {
			log.Warn("Jaeger set as OpenTelemetry trace exporter, but no Jaeger endpoint configured.")
		} else {
			tp, err := jaegerTracerProvider(jaegerEndpoint)
			if err != nil {
				log.Fatal(err)
			}
			tracerProvider = tp
			otel.SetTracerProvider(tp)
			otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
		}
	}

	if tracerProvider != nil {
		tracerCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		defer func(tracerContext context.Context) {
			ctx, cancel := context.WithTimeout(tracerContext, time.Second*5)
			defer cancel()
			if err := tracerProvider.Shutdown(ctx); err != nil {
				log.Fatal(err)
			}
		}(tracerCtx)
	}

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
