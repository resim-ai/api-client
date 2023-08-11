package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/resim-ai/api-client/api"
	"github.com/spf13/cobra"
)

var (
	experienceCmd = &cobra.Command{
		Use:   "experience",
		Short: "experience contains commands for creating and managing experiences",
		Long:  ``,
	}
	createExperienceCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates a new experience",
		Long:  ``,
		Run:   createExperience,
	}

	experienceName        string
	experienceDescription string
	experienceLocation    string
	experienceGithub      bool
)

func init() {
	createExperienceCmd.Flags().StringVar(&experienceName, "name", "", "The name of the experience")
	createExperienceCmd.Flags().StringVar(&experienceDescription, "description", "", "The description of the experience")
	createExperienceCmd.Flags().StringVar(&experienceLocation, "location", "", "The location of the experience, e.g. an S3 URI for the experience folder")
	createExperienceCmd.Flags().BoolVar(&experienceGithub, "github", false, "Whether to output format in github action friendly format")
	experienceCmd.AddCommand(createExperienceCmd)
	rootCmd.AddCommand(experienceCmd)
}

func createExperience(ccmd *cobra.Command, args []string) {
	if !experienceGithub {
		fmt.Println("Creating a experience...")
	}

	client, err := GetClient(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// Parse the various arguments from command line
	if experienceName == "" {
		log.Fatal("empty experience name")
	}

	if experienceDescription == "" {
		log.Fatal("empty experience description")
	}

	body := api.CreateExperienceJSONRequestBody{
		Name:        &experienceName,
		Description: &experienceDescription,
		Location:    &experienceLocation,
	}

	response, err := client.CreateExperienceWithResponse(context.Background(), body)
	if err != nil || response.StatusCode() != http.StatusCreated {
		log.Fatal("failed to create experience: ", err, string(response.Body))
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
