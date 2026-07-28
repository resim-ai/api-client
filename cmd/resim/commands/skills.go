package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/cmd/resim/commands/utils"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	skillCmd = &cobra.Command{
		Use:     "skills",
		Short:   "skills contains commands for registering and managing detection skills",
		Long:    ``,
		Aliases: []string{"skill"},
	}
	registerSkillCmd = &cobra.Command{
		Use:   "register",
		Short: "register - Registers an event-detection skill for a project",
		Long: `Registers an event-detection skill: a named, reusable prompt fragment the
agentic triage build feeds Claude when the skill is selected as an Analysis's
tier-a skill input.`,
		Run: registerSkill,
	}
)

const (
	skillProjectKey        = "project"
	skillNameKey           = "name"
	skillDescriptionKey    = "description"
	skillPromptFragmentKey = "prompt-fragment"
	skillGithubKey         = "github"
)

func init() {
	registerSkillCmd.Flags().String(skillProjectKey, "", "The name or ID of the project")
	registerSkillCmd.MarkFlagRequired(skillProjectKey)
	registerSkillCmd.Flags().String(skillNameKey, "", "A name for the skill")
	registerSkillCmd.MarkFlagRequired(skillNameKey)
	registerSkillCmd.Flags().String(skillDescriptionKey, "", "A description of the skill")
	registerSkillCmd.Flags().String(skillPromptFragmentKey, "", "The text the agentic triage build feeds Claude when this skill is selected")
	registerSkillCmd.Flags().Bool(skillGithubKey, false, "Whether to output format in github action friendly format")
	skillCmd.AddCommand(registerSkillCmd)

	rootCmd.AddCommand(skillCmd)
}

func registerSkill(ccmd *cobra.Command, args []string) {
	skillGithub := viper.GetBool(skillGithubKey)
	if !skillGithub {
		fmt.Println("Registering a skill...")
	}

	projectID := getProjectID(Client, viper.GetString(skillProjectKey))

	name := viper.GetString(skillNameKey)
	if name == "" {
		log.Fatal("empty skill name")
	}

	body := api.CreateSkillInput{
		Name: name,
	}
	if viper.IsSet(skillDescriptionKey) {
		body.Description = Ptr(viper.GetString(skillDescriptionKey))
	}
	if viper.IsSet(skillPromptFragmentKey) {
		body.PromptFragment = Ptr(viper.GetString(skillPromptFragmentKey))
	}

	response, err := Client.RegisterSkillWithResponse(context.Background(), projectID, body)
	if err != nil {
		log.Fatal("failed to register skill: ", err)
	}
	ValidateResponse(http.StatusCreated, "failed to register skill", response.HTTPResponse, response.Body)
	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	skill := *response.JSON201

	if skillGithub {
		fmt.Printf("skill_id=%s\n", skill.SkillID.String())
	} else {
		fmt.Println("Registered skill successfully!")
		fmt.Printf("Skill ID: %s\n", skill.SkillID.String())
	}
}
