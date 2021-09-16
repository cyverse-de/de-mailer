package main

import (
	"net/http"

	"github.com/cyverse-de/logcabin"
)

// API represents a single instance of the REST API.
type API struct {
	emailClient *EmailClient
	deSettings  *DESettings
}

func NewAPI(emailClient *EmailClient, deSettings *DESettings) *API {
	return &API{
		emailClient: emailClient,
		deSettings:  deSettings,
	}
}

// handles email notification requests
func (a *API) EmailRequestHandler(w http.ResponseWriter, r *http.Request) {
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
			//w.WriteHeader(getErrorResponseCode(err))
			//w.Write([]byte(err.Error()))
			JSONError(w, r, err.Error(), getErrorResponseCode(err))
			return
		} else {
			if emailReq.FromAddr == "" {
				emailReq.FromAddr = a.emailClient.fromAddress
			}
			formattedMsg, isHtml, err := FormatMessage(emailReq, payloadMap, *a.deSettings)
			if err != nil {
				logcabin.Error.Println(err)
				//	w.WriteHeader(http.StatusInternalServerError)
				//	w.Write([]byte(err.Error()))
				JSONError(w, r, err.Error(), getErrorResponseCode(err))
				return
			} else {
				toAddr := emailReq.To
				logcabin.Info.Println("Emailing " + toAddr + " host:" + a.emailClient.smtpHost)
				var mimeType string = TEXT_MIME_TYPE
				if isHtml {
					mimeType = HTML_MIME_TYPE
				}
				err := a.emailClient.Send([]string{toAddr}, mimeType, emailReq.Subject, formattedMsg.String())
				if err != nil {
					logcabin.Error.Println("failed to send email to " + toAddr + " host:" + a.emailClient.smtpHost)
				}
				w.WriteHeader(200)
				w.Write([]byte("Request processed successfully."))
				return
			}
		}

	default:
		logcabin.Error.Println("Unsupported request method.")
		w.WriteHeader(405)
		w.Write([]byte("Unsupported request method."))
		JSONError(w, r, "Unsupported request method.", 405)
		return
	}

}
