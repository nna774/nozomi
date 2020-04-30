package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/go-github/v31/github"
	"golang.org/x/oauth2"
)

// AccessToken is GitHub token
var AccessToken = os.Getenv("GitHubToken")

// SigningSecrets is
var SigningSecrets = os.Getenv("SigningSecrets")

// AllowedTeamID is
var AllowedTeamID = os.Getenv("AllowedTeamID")

// Input is the input of this lambda
type Input struct {
	Method  string            `json:"method"`
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
	Type    string            `json:"type"`
}

// Text is
type Text struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Emoji bool   `json:"emoji,omitempty"`
}

// Accessory is
type Accessory struct {
	Type  string `json:"type"`
	Text  Text   `json:"text"`
	Value string `json:"value"`
}

type block struct {
	Type      string     `json:"type"`
	Text      Text       `json:"text"`
	Accessory *Accessory `json:"accessory,omitempty"`
}

type body struct {
	ResponseType string   `json:"response_type,omitempty"`
	Blocks       *[]block `json:"blocks,omitempty"`
	Text         string   `json:"text,omitempty"`
}

// Response is the response of thi lambda
type Response struct {
	Body body `json:"body"`
}

func authBySigningSecrets(input Input) bool {
	ts := input.Headers["X-Slack-Request-Timestamp"]
	its, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return false
	}
	now := time.Now().Unix()
	if now-its > 60*5 {
		// replay attack
		return false
	}
	baseStr := "v0:" + ts + ":" + input.Body
	hmFunc := func(key, str string) string {
		mac := hmac.New(sha256.New, []byte(key))
		mac.Write([]byte(str))
		return hex.EncodeToString(mac.Sum(nil))
	}
	mySig := "v0=" + hmFunc(SigningSecrets, baseStr)
	slackSig := input.Headers["X-Slack-Signature"]
	return mySig == slackSig
}

func parseRequestBody(body string) map[string]string {
	s := strings.Split(body, "&")
	m := map[string]string{}
	for _, pair := range s {
		p := strings.SplitN(pair, "=", 2)
		if len(p) == 2 {
			unescaped, _ := url.QueryUnescape(p[1])
			m[p[0]] = unescaped
		} else {
			m[p[0]] = ""
		}
	}
	return m
}

func returnString(text string) (Response, error) {
	return Response{
		Body: body{
			ResponseType: "in_channel",
			Blocks: &[]block{
				{
					Type: "section",
					Text: Text{
						Type: "mrkdwn",
						Text: text,
					},
				},
			},
		},
	}, nil
}

func echo(req map[string]string, rest string) (Response, error) {
	return returnString(rest)
}

func showUsage() (Response, error) {
	return returnString("usage")
}

func showTestButton() (Response, error) {
	return Response{
		Body: body{
			ResponseType: "in_channel",
			Blocks: &[]block{
				{
					Type: "section",
					Text: Text{
						Type: "mrkdwn",
						Text: "piyopioyo",
					},
					Accessory: &Accessory{
						Type: "button",
						Text: Text{
							Type:  "plain_text",
							Text:  "push!",
							Emoji: true,
						},
						Value: "test",
					},
				},
			},
		},
	}, nil
}

func slash(input Input, req map[string]string) (Response, error) {
	if req["team_id"] != AllowedTeamID {
		return Response{}, errors.New("unallowed")
	}

	text := req["text"]
	ts := strings.SplitN(text, " ", 2)
	rest := ""
	if len(ts) == 2 {
		rest = ts[1]
	}
	switch ts[0] {
	case "showTestButton":
		return showTestButton()
	case "createTestIssue":
		return createTestIssue(req, rest)
	case "echo":
		return echo(req, rest)
	default:
		return showUsage()
	}
}

func nozomiHandler(ctx context.Context, input Input) (Response, error) {
	if !authBySigningSecrets(input) {
		return Response{}, errors.New("SigningSecret")
	}

	req := parseRequestBody(input.Body)
	switch input.Type {
	case "interactive":
		return interactive(input, req)
	case "select":
		fallthrough // uso
	case "slash":
		fallthrough
	default:
		return slash(input, req)
	}
}

// Interactive is
type Interactive struct {
	ResponseURI string `json:"response_url"`
}

func interactive(input Input, req map[string]string) (Response, error) {
	go func(req map[string]string) {
		var i Interactive
		json.Unmarshal([]byte(req["payload"]), &i)
		b, _ := json.Marshal(body{Text: "pushed!"})
		http.Post(i.ResponseURI, "application/json", bytes.NewReader(b))
	}(req)
	return Response{}, nil
}

func createTestIssue(req map[string]string, rest string) (Response, error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: AccessToken}, // scope: [repo]
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	rs := strings.SplitN(rest, " ", 2)
	if len(rs) < 2 {
		return returnString("/nozomi createTestIssue title body")
	}
	title := rs[0]
	body := rs[1]
	issue, _, err := client.Issues.Create(
		ctx,
		"kmc-jp",
		"test-repository",
		&github.IssueRequest{
			Title: &title,
			Body:  &body,
		},
	)

	if err != nil {
		return returnString(fmt.Sprintf("bie: %v\n", err))
	}

	return returnString(fmt.Sprintf("here! %s", *issue.HTMLURL))
}

func main() {
	lambda.Start(nozomiHandler)
}
