# Email Resource

A [Concourse](http://concourse.ci) resource that sends emails.

## Getting started
Add the following [Resource Type](http://concourse.ci/configuring-resource-types.html) to your Concourse pipeline
```yaml
resource_types:
  - name: email
    type: docker-image
    source:
      repository: pcfseceng/email-resource
```

Look at the [demo pipeline](https://github.com/pivotal-cf/email-resource/blob/master/ci/demo-pipeline.yml) for a complete example.

This resource acts as an SMTP client, using `PLAIN` auth over TLS.  So you need an SMTP server that supports all that.

For development, we've been using [Amazon SES](https://aws.amazon.com/ses/) with its [SMTP support](http://docs.aws.amazon.com/ses/latest/DeveloperGuide/smtp-credentials.html)

## Source Configuration
An example source configuration is below.  None of the parameters are optional.
```yaml
resources:
- name: send-an-email
  type: email
  source:
    smtp:
      host: smtp.example.com
      port: "587" # this must be a string
      username: a-user
      password: my-password
    from: build-system@example.com
    to: [ "dev-team@example.com", "product@example.net" ]
```

An example source configuration is below supporting sending email when anonymous is permitted.
```yaml
resources:
- name: send-an-email
  type: email
  source:
    smtp:
      host: smtp.example.com
      port: "587" # this must be a string
      anonymous: true
    from: build-system@example.com
    to: [ "dev-team@example.com", "product@example.net" ]
```
Note that `to` is an array, and that `port` is a string.
If you're using `fly configure` with the `--load-vars-from` (`-l`) substitutions, every `{{ variable }}`
[automatically gets converted to a string](http://concourse.ci/fly-cli.html).
But for literals you need to surround it with quotes.

## Behavior

This is an output-only resource, so `check` and `in` actions are no-ops.

### `out`: Send an email

#### Parameters

* `headers`: *Optional.* Path to plain text file containing additional mail headers
* `subject`: *Required.* Path to plain text file containing the subject
* `body`: *Required.* Path to file containing the email body.
* `send_empty_body`: *Optional.* If true, send the email even if the body is empty (defaults to `false`).
* `istemplatesubject` *Optional.* If true, subject file is assumed to contain concourse env variables to be replaced  (defaults to `false`).
* `istemplatebody` *Optional.* If true, body file assumed to contain concourse en variables to be replaced  (defaults to `false`).


A build plan might contain this:
```yaml
  - put: send-an-email
    params:
      subject: demo-prep-sha-email/generated-subject
      body: demo-prep-sha-email/generated-body
```

If using template files, the generated subject and body files should contain place holders to be replaced with `Concourse` build metadata. For eg, a generated template file with Concourse metadata place holders would look something like this :


```echo "Build {{.BuildJobName }} of {{.BuildPipelineName}} pipeline failed" >> ./emailout/email-subject-failure.txt```

```echo "Build {{.BuildName }} of job {{.BuildJobName }} for pipeline {{.BuildPipelineName}} failed." >> ./emailout/email-body-failure.txt```

```echo "Please see the build details here : {{.ExternalURL }}/teams/{{.BuildTeamName}}/pipelines/{{.BuildPipelineName}}/jobs/{{.BuildJobName}}/builds/{{.BuildName}}" >> ./emailout/email-body-failure.txt```



#### HTML Email

To send HTML email set the `headers` parameter to a file containing the following:

```
MIME-version: 1.0
Content-Type: text/html; charset="UTF-8"
```
