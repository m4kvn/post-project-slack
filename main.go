package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/nlopes/slack"
	"golang.org/x/oauth2"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
)

type Env struct {
	Token   string
	Webhook string
}

type User struct {
	GitHub string `json:"github"`
	Slack  string `json:"slack"`
}

type Setting struct {
	Users      []User   `json:"users"`
	Columns    []string `json:"columns"`
	Owner      string   `json:"owner"`
	Repository string   `json:"repository"`
	Username   string   `json:"username"`
}

func getEnv() Env {
	return Env{
		Token:   os.Getenv("TOKEN_GITHUB"),
		Webhook: os.Getenv("WEBHOOK_SLACK"),
	}
}

func readSetting(fileName string) Setting {
	bytes, _ := ioutil.ReadFile(fileName)
	var setting Setting
	_ = json.Unmarshal(bytes, &setting)
	for _, u := range setting.Users {
		fmt.Println(u)
	}
	return setting
}

func getSettingFileName() string {
	settingFile := flag.String("f", "setting.json", "Setting json file")
	flag.Parse()
	return *settingFile
}

func main() {
	env := getEnv()
	setting := readSetting(getSettingFileName())
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: env.Token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	projects, _, _ := client.Repositories.ListProjects(ctx, setting.Owner, setting.Repository, nil)

	for _, project := range projects {
		message := slack.WebhookMessage{
			Username:    setting.Username,
			Attachments: []slack.Attachment{},
		}

		cols, _, _ := client.Projects.ListProjectColumns(ctx, project.GetID(), nil)

	COLUMNS:
		for _, col := range cols {
			var tasks []string
			skip := true

			for _, column := range setting.Columns {
				if regexp.MustCompile(column).MatchString(col.GetName()) {
					skip = false
				}
			}

			if skip {
				fmt.Print("skip: ")
				fmt.Println(col.GetName())
				continue COLUMNS
			}

			attachment := slack.Attachment{
				Title: col.GetName(),
			}

			cards, _, _ := client.Projects.ListProjectCards(ctx, col.GetID(), nil)

			for _, card := range cards {
				contentUrl, _ := url.Parse(card.GetContentURL())
				number, _ := strconv.Atoi(path.Base(contentUrl.Path))
				issue, _, _ := client.Issues.Get(ctx, setting.Owner, setting.Repository, number)
				userUrl := issue.GetAssignee().GetHTMLURL()
				userName := path.Base(userUrl)

				var name string
				for _, user := range setting.Users {
					if user.GitHub == userName {
						name = "<@" + user.Slack + ">"
					}
				}

				if name == "" {
					name = "No assignee"
				}

				t := "◎ <" + issue.GetHTMLURL() + "|" + issue.GetTitle() + "> (" + name + ")\n"

				tasks = append(tasks, t)
			}

			var text strings.Builder
			for _, task := range tasks {
				text.WriteString(task)
			}
			attachment.Text = text.String()

			message.Attachments = append(message.Attachments, attachment)
		}

		_ = slack.PostWebhook(env.Webhook, &message)
	}
}
