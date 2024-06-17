package main

import (
	"log"
	"io/ioutil"
	"fmt"
	"gopkg.in/yaml.v2"
	"net/http"
	"encoding/json"
	"bytes"
	"time"
	"os/exec"
)

type Config struct {
	Contacts   []string `yaml:"contacts"`
	Containerid      string `yaml:"containerid"`
	Restarturl       string `yaml:"restarturl"`
	Alertname       string `yaml:"alertname"`
	Runcommands     bool `yaml:"runcommands"`
	Commands     []string `yaml:"commands"`
	RootPassword  string `yaml:"rootpassword"`
	ServerIP      string `yaml:"serverip"`
	Smsgateway struct {
		URL      string `yaml:"url"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"smsgateway"`

}

type PrometheusRequests struct {
	Receiver string `json:"receiver"`
	Status string `json:"status"`
	Alerts []struct {
		Status string `json:"status"`
		Labels struct {
			Alertname string `json:"alertname"`
			Config string `json:"command"`
			Instance string `json:"instance"`
			Job string `json:"job"`
			Severity string `json:"severity"`
		} `json:"labels"`
		Annotations struct {
			Summary string `json:"summary"`
		} `json:"annotations"`
		StartsAt time.Time `json:"startsAt"`
		EndsAt time.Time `json:"endsAt"`
		GeneratorURL string `json:"generatorURL"`
		Fingerprint string `json:"fingerprint"`
	} `json:"alerts"`
	GroupLabels struct {
		Alertname string `json:"alertname"`
		Instance string `json:"instance"`
	} `json:"groupLabels"`
	CommonLabels struct {
		Alertname string `json:"alertname"`
		Config string `json:"command"`
		Instance string `json:"instance"`
		Job string `json:"job"`
		Severity string `json:"severity"`
	} `json:"commonLabels"`
	CommonAnnotations struct {
		Summary string `json:"summary"`
	} `json:"commonAnnotations"`
	ExternalURL string `json:"externalURL"`
	Version string `json:"version"`
	GroupKey string `json:"groupKey"`
	TruncatedAlerts int `json:"truncatedAlerts"`
}

type GrafanaRequests struct {
	Title string `json:"title"`
	RuleID int `json:"ruleId"`
	RuleName string `json:"ruleName"`
	State string `json:"state"`
	EvalMatches []interface{} `json:"evalMatches"`
	OrgID int `json:"orgId"`
	DashboardID int `json:"dashboardId"`
	PanelID int `json:"panelId"`
	Tags struct {
	} `json:"tags"`
	RuleURL string `json:"ruleUrl"`
	Message string `json:"message"`
}

type SMSRequest struct {
	Message     string `json:"Message"`
	PhoneNumber string `json:"PhoneNumber"`
}

func main() {
	data, err := ioutil.ReadFile("configs.yaml")
	if err != nil {
		log.Fatalf("Error reading YAML file: %v", err)
	}

	// Parse the YAML file into a slice of Config structs
	var configs Config
	err = yaml.Unmarshal(data, &configs)
	if err != nil {
		log.Fatalf("Error unmarshaling YAML data: %v", err)
	}
	// Set up a HTTP server to recive requests
	mux := http.NewServeMux()
	mux.HandleFunc("/grafana", func(w http.ResponseWriter, r *http.Request) {
        	grafanaAlertingHandler(w, r, configs)
        })
	mux.HandleFunc("/alertmanager", func(w http.ResponseWriter, r *http.Request) {
        	prometheusAlertingHandler(w, r, configs)
        })
	log.Fatal(http.ListenAndServe(":5000", mux))
}


func grafanaAlertingHandler(w http.ResponseWriter, r *http.Request, configs Config) {
	// Ensure the request method is POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the body of the request
	defer r.Body.Close()

	// Unmarshal the JSON data
	var alertRequest GrafanaRequests
	if err := json.NewDecoder(r.Body).Decode(&alertRequest); err != nil {
		http.Error(w, "Bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Process the data (example: just print it)
	fmt.Printf("Received request: %+v\n", alertRequest)

	// Respond to the client
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Alert/Resolve received successfully"))
	finalMessage := ""
	if alertRequest.State == "ok" {
		finalMessage = fmt.Sprintf("*Resolve Message*\nTitle: %s\nDescription: %s\nState: %s",alertRequest.Title,alertRequest.Message,alertRequest.State)
	} else if alertRequest.State == "alerting" {
		finalMessage = fmt.Sprintf("*Alerting Message*\nTitle: %s\nDescription: %s\nState: %s",alertRequest.Title,alertRequest.Message,alertRequest.State)
	} else {
		finalMessage = "I can not find alert state .Please check your Application"
	}
	for _, phoneNumber := range configs.Contacts {
		sendSMS(finalMessage, phoneNumber, configs.Smsgateway.URL, configs.Smsgateway.Username, configs.Smsgateway.Password)
	}
}


func prometheusAlertingHandler(w http.ResponseWriter, r *http.Request, configs Config) {
	// Ensure the request method is POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the body of the request
	defer r.Body.Close()

	// Unmarshal the JSON data
	var alertRequest PrometheusRequests
	if err := json.NewDecoder(r.Body).Decode(&alertRequest); err != nil {
		http.Error(w, "Bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Printf("Received request: %+v\n", alertRequest)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Alert/Resolve received successfully"))
	finalMessage := ""
	if alertRequest.Status == "resolved" {
		finalMessage = fmt.Sprintf("*Resolve Message*\nAlertName: %s\nDescription: %s", alertRequest.Alerts[0].Labels.Alertname, alertRequest.Alerts[0].Annotations.Summary)
	} else if alertRequest.Status == "firing" {
		finalMessage = fmt.Sprintf("*Alerting Message*\nAlertName: %s\nDescription: %s", alertRequest.Alerts[0].Labels.Alertname, alertRequest.Alerts[0].Annotations.Summary)
	} else {
		finalMessage = "I can not find alert state .Please check your Application"
	}
	for _, phoneNumber := range configs.Contacts {
		sendSMS(finalMessage, phoneNumber, configs.Smsgateway.URL, configs.Smsgateway.Username, configs.Smsgateway.Password)
		if alertRequest.Alerts[0].Labels.Alertname == configs.Alertname && configs.Runcommands {
			for _,command := range configs.Commands {
				cmd := exec.Command("sh", "-c", "sshpass -p '"+ configs.RootPassword +"' ssh -o StrictHostKeyChecking=no root@"+ configs.ServerIP + """ + command + """ )
				err := cmd.Run()
				if err != nil {
					log.Println("Error running command:", err, "on command ", command)
				} else {
					log.Println("COMMAND : ",command , " runned successfully")
				}
		}
			
			
		}
	}
}



func sendSMS(message, phoneNumber, URL, smsUsername, smsPassword string) error {
	// Define the URL
	url := URL

	// Create the JSON payload
	smsRequest := SMSRequest{
		Message:     message,
		PhoneNumber: phoneNumber,
	}
	payload, err := json.Marshal(smsRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	// Create a new HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Set the headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("UserName", smsUsername)
	req.Header.Set("Password", smsPassword)

	// Create a new HTTP client and send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-OK response status: %v", resp.Status)
	}
	log.Println(resp.StatusCode, smsRequest , resp)

	return nil
}
