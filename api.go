package main

import (
	"net/http"
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
	ctx := r.Context()
	if r.URL.Path != "/" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}
	switch r.Method {
	case "GET":
		w.WriteHeader(200)
		w.Write([]byte("A service that handles email-requests and send out emails to users.")) // nolint:errcheck
	case "POST":
		log.Info("Post request received.")
		emailReq, payloadMap, err := parseRequestBody(r)
		if err != nil {
			log.Error(err)
			//w.WriteHeader(getErrorResponseCode(err))
			//w.Write([]byte(err.Error()))
			JSONError(w, r, err.Error(), getErrorResponseCode(err))
			return
		} else {
			if emailReq.FromAddr == "" {
				emailReq.FromAddr = a.emailClient.fromAddress
			}
			formattedMsg, isHtml, err := FormatMessage(ctx, emailReq, payloadMap, *a.deSettings)
			if err != nil {
				log.Error(err)
				//	w.WriteHeader(http.StatusInternalServerError)
				//	w.Write([]byte(err.Error()))
				JSONError(w, r, err.Error(), getErrorResponseCode(err))
				return
			} else {
				toAddr := emailReq.To
				log.Infof("Emailing %s host: %s", toAddr, a.emailClient.smtpHost)
				var mimeType = TEXT_MIME_TYPE
				if isHtml {
					mimeType = HTML_MIME_TYPE
				}
				formattedReq := &FormattedEmailRequest{
					To:       []string{toAddr},
					Cc:       emailReq.Cc,
					Bcc:      emailReq.Bcc,
					From:     emailReq.FromAddr,
					Subject:  emailReq.Subject,
					MIMEType: mimeType,
					Body:     formattedMsg.String(),
				}
				err := a.emailClient.Send(ctx, formattedReq)
				if err != nil {
					log.Error("failed to send email to " + toAddr + " host:" + a.emailClient.smtpHost)
				}
				w.WriteHeader(200)
				_, err = w.Write([]byte("Request processed successfully."))
				if err != nil {
					log.Errorf("Failed to send response: %s\n", err)
				}
				return
			}
		}

	default:
		log.Error("Unsupported request method.")
		w.WriteHeader(405)
		w.Write([]byte("Unsupported request method.")) // nolint:errcheck
		JSONError(w, r, "Unsupported request method.", 405)
		return
	}

}
