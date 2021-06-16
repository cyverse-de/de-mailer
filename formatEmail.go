package main

import (
	"bytes"
	"encoding/json"
	"html/template"

	"github.com/cyverse-de/logcabin"
)

// message in the email request
type message struct {
	Text string
}

// email request received
type EmailRequest struct {
	User           string
	Email_Template string
	Subject        string
	Message        message
	Payload        json.RawMessage
}

//format email message using templates
func FormatEmail(emailReq EmailRequest, payload map[string](interface{})) (bytes.Buffer, error) {
	logcabin.Info.Println("Received formatting request for template " + emailReq.Email_Template + " for user " + payload["user"].(string))
	tmpl, err := template.ParseFiles("./templates/"+emailReq.Email_Template+".tmpl", "./templates/header.tmpl", "./templates/footer.tmpl")
	var template_output bytes.Buffer
	if err != nil {
		logcabin.Error.Fatal(err)
		return template_output, err
	}

	data := struct {
		User                 string
		AnalysisName         string
		AnalysisStatus       string
		AnalysisDescription  string
		AnalysisStartDate    string
		AnalysisResultFolder string
	}{User: payload["user"].(string),
		AnalysisName:         payload["analysisname"].(string),
		AnalysisStatus:       payload["analysisstatus"].(string),
		AnalysisDescription:  payload["analysisdescription"].(string),
		AnalysisStartDate:    payload["startdate"].(string),
		AnalysisResultFolder: "https://de.cyverse.org/data/ds/iplant/home/sriram/analyses",
	}

	tmpl_err := tmpl.Execute(&template_output, data)
	if tmpl_err != nil {
		logcabin.Error.Fatal(tmpl_err)
	}
	return template_output, tmpl_err
}
