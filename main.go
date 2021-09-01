package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/DavidGamba/go-getoptions"
	"github.com/cyverse-de/configurate"
	"github.com/cyverse-de/logcabin"
)

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
		fmt.Fprintf(os.Stderr, opt.Help())
		os.Exit(0)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n\n", err)
		fmt.Fprintf(os.Stderr, opt.Help(getoptions.HelpSynopsis))
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
		logcabin.Error.Println(msg)
		return emailReq, payloadMap, NewHTTPError(http.StatusInternalServerError, msg)
	}
	fmt.Println(string(body))

	err = json.Unmarshal(body, &emailReq)

	if err != nil {
		msg := fmt.Sprintf("failed to parse request body: %s", err.Error())
		logcabin.Error.Println(msg)
		return emailReq, payloadMap, NewHTTPError(http.StatusBadRequest, msg)
	} else {
		err := json.Unmarshal(emailReq.Values, &payloadMap)
		if err != nil {
			msg := fmt.Sprintf("failed to parse template values: %s", err.Error())
			logcabin.Error.Println(msg)
			return emailReq, payloadMap, NewHTTPError(http.StatusBadRequest, msg)
		}
		return emailReq, payloadMap, err
	}
}

func main() {
	// Initialize logging.
	logcabin.Init("de-mailer", "de-mailer")

	// Parse the command line.
	optionValues := parseCommandLine()

	// Load the configuration.
	config, err := configurate.InitDefaults(optionValues.Config, configurate.JobServicesDefaults)
	if err != nil {
		logcabin.Error.Fatal(err)
	}
	//test config
	logcabin.Info.Println(config.GetString("de.base"))

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
	http.HandleFunc("/", api.EmailRequestHandler)
	logcabin.Error.Fatal(http.ListenAndServe(":8080", nil))
	logcabin.Error.Fatal(http.ListenAndServe(":8080", nil))
}
