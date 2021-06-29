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
	communities string
	apps        string
	admin       string
	doi         string
	vice        string
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
		logcabin.Error.Println(err)
		return emailReq, payloadMap, err
	}
	logcabin.Info.Println(string(body))

	err = json.Unmarshal(body, &emailReq)

	if err != nil {
		logcabin.Error.Println(err)
		return emailReq, payloadMap, err
	} else {
		err := json.Unmarshal(emailReq.Values, &payloadMap)
		if err != nil {
			logcabin.Error.Println(err)
		}
		return emailReq, payloadMap, err

	}
}

// handles email notification requests
func EmailRequestHandler(w http.ResponseWriter, r *http.Request, emailSettings EmailSettings, deSettings DESettings) {
	if r.URL.Path != "/" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}
	switch r.Method {
	case "GET":
		w.WriteHeader(200)
		w.Write([]byte("A service that handles email-requests and send out emails to users."))
	case "POST":
		logcabin.Info.Println("Post request received.")
		emailReq, payloadMap, err := parseRequestBody(r)
		if err != nil {
			logcabin.Error.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		} else {
			formattedMsg, err := FormatMessage(emailReq, payloadMap, deSettings)
			if err != nil {
				logcabin.Error.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			} else {
				toAddr := payloadMap["email_address"].(string)
				r := NewEmail(emailSettings.smtpHost, emailSettings.fromAddress, []string{toAddr}, emailReq.Subject, formattedMsg.String())
				logcabin.Info.Println("Emailing " + toAddr + " host:" + emailSettings.smtpHost)
				ok, _ := r.SendEmail()
				fmt.Println(ok)
				w.WriteHeader(200)
				w.Write([]byte("Request processed successfully."))
				return
			}
		}

	default:
		logcabin.Error.Println("Unsupported request method.")
		w.WriteHeader(405)
		w.Write([]byte("Unsupported request method."))
		return
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

	emailSettings := EmailSettings{
		smtpHost:    config.GetString("email.smtpHost"),
		fromAddress: config.GetString("email.fromAddress"),
	}

	deSettings := DESettings{
		base:        config.GetString("de.base"),
		data:        config.GetString("de.data"),
		analyses:    config.GetString("de.analyses"),
		teams:       config.GetString("teams"),
		tools:       config.GetString("tools"),
		communities: config.GetString("communities"),
		apps:        config.GetString("apps"),
		admin:       config.GetString("admin"),
		doi:         config.GetString("doi"),
		vice:        config.GetString("vice"),
	}

	http.HandleFunc("/", func(writer http.ResponseWriter, reader *http.Request) {
		EmailRequestHandler(writer, reader, emailSettings, deSettings)
	})
	logcabin.Error.Fatal(http.ListenAndServe(":8080", nil))
}
