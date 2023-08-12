package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/resim-ai/api-client/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	experienceCmd = &cobra.Command{
		Use:   "experience",
		Short: "experience contains commands for creating and managing experiences",
		Long:  ``,
	}
	createExperienceCmd = &cobra.Command{
		Use:    "create",
		Short:  "create - Creates a new experience",
		Long:   ``,
		Run:    createExperience,
		PreRun: RegisterViperFlags,
	}
)

const (
	experienceNameKey        = "name"
	experienceDescriptionKey = "description"
	experienceLocationKey    = "location"
	experienceGithubKey      = "github"
)

func init() {
	createExperienceCmd.Flags().String(experienceNameKey, "", "The name of the experience")
	createExperienceCmd.Flags().String(experienceDescriptionKey, "", "The description of the experience")
	createExperienceCmd.Flags().String(experienceLocationKey, "", "The location of the experience, e.g. an S3 URI for the experience folder")
	createExperienceCmd.Flags().Bool(experienceGithubKey, false, "Whether to output format in github action friendly format")
	experienceCmd.AddCommand(createExperienceCmd)
	rootCmd.AddCommand(experienceCmd)
}

func createExperience(ccmd *cobra.Command, args []string) {
	experienceGithub := viper.GetBool(experienceGithubKey)
	if !experienceGithub {
		fmt.Println("Creating an experience...")
	}

	client, err := GetClient(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// Parse the various arguments from command line
	experienceName := viper.GetString(experienceNameKey)
	if experienceName == "" {
		log.Fatal("empty experience name")
	}

	experienceDescription := viper.GetString(experienceDescriptionKey)
	if experienceDescription == "" {
		log.Fatal("empty experience description")
	}

	experienceLocation := viper.GetString(experienceLocationKey)
	if experienceLocation == "" {
		log.Fatal("empty experience location")
	}

	body := api.CreateExperienceJSONRequestBody{
		Name:        &experienceName,
		Description: &experienceDescription,
		Location:    &experienceLocation,
	}

	response, err := client.CreateExperienceWithResponse(context.Background(), body)
	if err != nil || response.StatusCode() != http.StatusCreated {
		var message string
		if response != nil && response.Body != nil {
			message = string(response.Body)
		}
		log.Fatal("failed to create experience: ", err, message)
	}
	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	experience := response.JSON201
	if experience.ExperienceID == nil {
		log.Fatal("no experience ID")
	}

	// Report the results back to the user
	if experienceGithub {
		fmt.Printf("experience_id=%s\n", experience.ExperienceID.String())
	} else {
		fmt.Println("Created experience successfully!")
		fmt.Printf("Experience ID: %s\n", experience.ExperienceID.String())
	}
}
