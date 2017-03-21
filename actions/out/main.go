package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	sourceRoot := os.Args[1]
	if sourceRoot == "" {
		fmt.Fprintf(os.Stderr, "expected path to build sources as first argument")
		os.Exit(1)
	}

	var indata struct {
		Source struct {
			SMTP struct {
				Host      string
				Port      string
				Username  string
				Password  string
				Anonymous bool `json:"anonymous"`
			}
			From string
			To   []string
		}
		Params struct {
			Subject         string
			TemplateSubject bool `json:"istemplatesubject"`
			Body            string
			TemplateBody    bool `json:"istemplatebody"`
			SendEmptyBody   bool `json:"send_empty_body"`
			Headers         string
		}
	}

	type subjectBuildParams struct {
		BuildJobName      string
		BuildPipelineName string
	}

	type bodyBuildParams struct {
		BuildName         string
		BuildJobName      string
		BuildPipelineName string
		BuildTeamName     string
		ExternalURL       string
	}

	inbytes, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(inbytes, &indata)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing input as JSON: %s", err)
		os.Exit(1)
	}

	if indata.Source.SMTP.Host == "" {
		fmt.Fprintf(os.Stderr, `missing required field "source.smtp.host"`)
		os.Exit(1)
	}

	if indata.Source.SMTP.Port == "" {
		fmt.Fprintf(os.Stderr, `missing required field "source.smtp.port"`)
		os.Exit(1)
	}

	if indata.Source.From == "" {
		fmt.Fprintf(os.Stderr, `missing required field "source.from"`)
		os.Exit(1)
	}

	if len(indata.Source.To) == 0 {
		fmt.Fprintf(os.Stderr, `missing required field "source.to"`)
		os.Exit(1)
	}

	if indata.Params.Subject == "" {
		fmt.Fprintf(os.Stderr, `Subjectfile needs to be specified`)
		os.Exit(1)
	}

	if indata.Source.SMTP.Anonymous == false {
		if indata.Source.SMTP.Username == "" {
			fmt.Fprintf(os.Stderr, `missing required field "source.smtp.username" if anonymous specify anonymous: true`)
			os.Exit(1)
		}

		if indata.Source.SMTP.Password == "" {
			fmt.Fprintf(os.Stderr, `missing required field "source.smtp.password" if anonymous specify anonymous: true`)
			os.Exit(1)
		}
	}

	convertTemplateText := func(sourcePath, destTemplateFile string, buildParams interface{}) (string, error) {
		tmpl, err := template.ParseFiles(sourcePath)
		if err != nil {
			return "", err
		}
		destTemplateFile = filepath.Join(sourceRoot, destTemplateFile)
		writer, err := os.Create(destTemplateFile)
		if err != nil {
			return "", err
		}
		defer writer.Close()
		err = tmpl.Execute(writer, buildParams)
		if err != nil {
			return "", err
		}
		bytes, err := ioutil.ReadFile(destTemplateFile)
		if err != nil {
			return "", err
		}
		fileText := string(bytes)
		//This print statement required so that the tests can grab the output
		fmt.Fprintf(os.Stdout, "Output : %s", fileText)
		return fileText, nil
	}

	readSource := func(sourcePath string) (string, error) {
		if !filepath.IsAbs(sourcePath) {
			sourcePath = filepath.Join(sourceRoot, sourcePath)
		}
		bytes, err := ioutil.ReadFile(sourcePath)
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}

	convertSubjectTemplate := func(sourcePath string) (string, error) {
		if !filepath.IsAbs(sourcePath) {
			sourcePath = filepath.Join(sourceRoot, sourcePath)
		}
		buildParams := subjectBuildParams{os.Getenv("BUILD_JOB_NAME"), os.Getenv("BUILD_PIPELINE_NAME")}
		return convertTemplateText(sourcePath, "subject_template.txt", buildParams)
	}

	convertBodyTemplate := func(sourcePath string) (string, error) {
		if !filepath.IsAbs(sourcePath) {
			sourcePath = filepath.Join(sourceRoot, sourcePath)
		}
		buildParams := bodyBuildParams{
			os.Getenv("BUILD_NAME"),
			os.Getenv("BUILD_JOB_NAME"),
			os.Getenv("BUILD_PIPELINE_NAME"),
			os.Getenv("BUILD_TEAM_NAME"),
			os.Getenv("ATC_EXTERNAL_URL"),
		}
		return convertTemplateText(sourcePath, "body_template.txt", buildParams)
	}

	var subject string

	if indata.Params.TemplateSubject {
		subject, err = convertSubjectTemplate(indata.Params.Subject)
	} else {
		subject, err = readSource(indata.Params.Subject)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
	subject = strings.Trim(subject, "\n")

	var headers string
	if indata.Params.Headers != "" {
		headers, err = readSource(indata.Params.Headers)
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
		headers = strings.Trim(headers, "\n")
	}

	var body string
	if indata.Params.Body != "" {
		if indata.Params.TemplateBody {
			body, err = convertBodyTemplate(indata.Params.Body)
		} else {
			body, err = readSource(indata.Params.Body)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
	}

	type MetadataItem struct {
		Name  string
		Value string
	}
	var outdata struct {
		Version struct {
			Time time.Time
		} `json:"version"`
		Metadata []MetadataItem
	}
	outdata.Version.Time = time.Now().UTC()
	outdata.Metadata = []MetadataItem{
		{Name: "smtp_host", Value: indata.Source.SMTP.Host},
		{Name: "subject", Value: subject},
	}
	outbytes, err := json.Marshal(outdata)
	if err != nil {
		panic(err)
	}

	var messageData []byte
	messageData = append(messageData, []byte("To: "+strings.Join(indata.Source.To, ", ")+"\n")...)
	if headers != "" {
		messageData = append(messageData, []byte(headers+"\n")...)
	}
	messageData = append(messageData, []byte("Subject: "+subject+"\n")...)

	messageData = append(messageData, []byte("\n")...)
	messageData = append(messageData, []byte(body)...)

	if indata.Params.SendEmptyBody == false && len(body) == 0 {
		fmt.Fprintf(os.Stderr, "Message not sent because the message body is empty and send_empty_body parameter was set to false. Github readme: https://github.com/pivotal-cf/email-resource")
		fmt.Printf("%s", []byte(outbytes))
		return
	}

	if indata.Source.SMTP.Anonymous {
		var c *smtp.Client
		var wc io.WriteCloser
		c, err = smtp.Dial(fmt.Sprintf("%s:%s", indata.Source.SMTP.Host, indata.Source.SMTP.Port))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error Dialing: "+err.Error())
			os.Exit(1)
		}
		defer c.Close()
		c.Mail(indata.Source.From)

		for _, toAddress := range indata.Source.To {
			c.Rcpt(toAddress)
		}
		// Send the email body.
		wc, err = c.Data()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error Getting writer context: "+err.Error())
			os.Exit(1)
		}
		defer wc.Close()
		_, err = wc.Write(messageData)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error Writing bytes: "+err.Error())
			os.Exit(1)
		}

	} else {
		err = smtp.SendMail(
			fmt.Sprintf("%s:%s", indata.Source.SMTP.Host, indata.Source.SMTP.Port),
			smtp.PlainAuth(
				"",
				indata.Source.SMTP.Username,
				indata.Source.SMTP.Password,
				indata.Source.SMTP.Host,
			),
			indata.Source.From,
			indata.Source.To,
			messageData,
		)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to send an email using SMTP server %s on port %s: %v",
			indata.Source.SMTP.Host, indata.Source.SMTP.Port, err)
		os.Exit(1)
	}

	fmt.Printf("%s", []byte(outbytes))
}
