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
		log.Fatal("Empty experience name")
	}

	if experienceDescription == "" {
		log.Fatal("Empty experience description")
	}

	body := api.CreateExperienceJSONRequestBody{
		Name:        &experienceName,
		Description: &experienceDescription,
		Location:    &experienceLocation,
	}

	response, err := client.CreateExperienceWithResponse(context.Background(), body)

	if err != nil {
		log.Fatal(err)
	}

	// Report the results back to the user
	success := response.HTTPResponse.StatusCode == http.StatusCreated
	if success {
		if experienceGithub {
			fmt.Printf("experience_id=%s\n", response.JSON201.ExperienceID.String())
		} else {
			fmt.Println("Created experience successfully!")
			fmt.Printf("Experience ID: %s\n", response.JSON201.ExperienceID.String())
		}
	} else {
		log.Fatal("Failed to create experience!\n", string(response.Body))
	}

}
