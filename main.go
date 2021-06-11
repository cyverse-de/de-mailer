package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"encoding/json"
	"io/ioutil"

	"github.com/DavidGamba/go-getoptions"
	"github.com/cyverse-de/configurate"
	"github.com/cyverse-de/logcabin"
)

type commandLineOptionValues struct {
	Config string
}

// message in the email request
type message struct {
	Text string
}

// email request received
type emailRequest struct {
	User           string
	Email_Template string
	Subject        string
	Message        message
	Payload        json.RawMessage
}

func buildEmailMessage(t emailRequest, payload map[string](interface{})) {
	tmpl, err := template.ParseFiles("./templates/"+t.Email_Template+".tmpl", "./templates/header.tmpl", "./templates/footer.tmpl")

	if err != nil {
		logcabin.Error.Fatal(err)
		return
	}

	data := struct {
		User                 string
		AnalysisName         string
		AnalysisStatus       string
		AnalysisDescription  string
		AnalysisStartDate    string
		AnalysisResultFolder string
	}{User: "sriram",
		AnalysisName:         "Test Analysis",
		AnalysisStatus:       "Running",
		AnalysisDescription:  "This is testing template",
		AnalysisStartDate:    "05/12/2021 00 00 00",
		AnalysisResultFolder: "https://de.cyverse.org/data/ds/iplant/home/sriram/analyses",
	}

	tmpl_err := tmpl.Execute(os.Stdout, data)
	if tmpl_err != nil {
		log.Fatalf("template execution: %s", tmpl_err)
	}
}

// handles email notification requests
func emailRequestHandler(w http.ResponseWriter, r *http.Request) {
	var t emailRequest

	if r.URL.Path != "/" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}
	switch r.Method {
	case "GET":
		fmt.Fprintf(w, "A service that handles email-requests and send out emails to users.")
	case "POST":
		logcabin.Info.Println("Post request received")
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logcabin.Error.Fatal(err)
			return
		}
		logcabin.Info.Println(string(body))

		err = json.Unmarshal(body, &t)
		if err != nil {
			logcabin.Error.Fatal(err)
			return
		} else {
			// unmarshell payload to map with interface{}
			jsonMap := make(map[string](interface{}))
			err := json.Unmarshal(t.Payload, &jsonMap)
			if err != nil {
				logcabin.Error.Fatal(err)
				return
			} else {
				logcabin.Info.Println(jsonMap["analysisresultsfolder"])
				buildEmailMessage(t, jsonMap)
			}
		}

	default:
		fmt.Fprintf(w, "Sorry, only GET and POST methods are supported.")
	}
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
	defaultConfigPath := "/etc/iplant/de/jobservices.yml"

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

	http.HandleFunc("/", emailRequestHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
