package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	dockerterm "github.com/moby/term"
	"github.com/resim-ai/api-client/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util/term"
)

var (
	debugCmd = &cobra.Command{
		Use:   "debug",
		Short: "debug - Launch an interactive debug session",
		Long:  ``,
		Run:   debug,
	}
)

const (
	debugProjectKey    = "project"
	debugBuildKey      = "build"
	debugExperienceKey = "experience"
)

func init() {
	debugCmd.Flags().String(debugProjectKey, "", "The name or ID of the project to associate with the debug session")
	debugCmd.MarkFlagRequired(debugProjectKey)
	debugCmd.Flags().String(debugBuildKey, "", "The ID of the build to debug")
	debugCmd.MarkFlagRequired(debugBuildKey)
	debugCmd.Flags().String(debugExperienceKey, "", "The experience to debug")
	debugCmd.MarkFlagRequired(debugExperienceKey)
	rootCmd.AddCommand(debugCmd)
}

func debug(ccmd *cobra.Command, args []string) {
	ctx := context.Background()
	projectID := getProjectID(Client, viper.GetString(debugProjectKey))
	buildIDstring := viper.GetString(debugBuildKey)

	buildID, err := uuid.Parse(buildIDstring)

	// pool label
	poolLabels := []string{"resim:k8s"}
	if err != nil {
		log.Fatal("invalid build ID: ", err)
	}

	experienceID := getExperienceID(Client, projectID, viper.GetString(debugExperienceKey), true)

	body := api.DebugExperienceJSONRequestBody{
		BuildID:    &buildID,
		PoolLabels: &poolLabels,
	}

	response, err := Client.DebugExperienceWithResponse(ctx, projectID, experienceID, body)
	if err != nil {
		log.Fatal("unable to debug experience: ", err)
	}

	ValidateResponse(http.StatusCreated, "unable to debug experience", response.HTTPResponse, response.Body)

	debugExperience := response.JSON201

	fmt.Println("Batch ID:", debugExperience.BatchID)
	fmt.Println("Waiting for debug environment to be ready...")

	command := "bash"

	serverAddress := debugExperience.ClusterEndpoint
	token := debugExperience.ClusterToken

	restConfig := rest.Config{
		Host:        *serverAddress,
		BearerToken: *token, // 15 minutes
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
			// TODO: Add CA data
			// CAData: []byte(*debugExperience.ClusterCAData),
		},
	}

	clientSet, err := kubernetes.NewForConfig(&restConfig)
	if err != nil {
		panic(err)
	}

	// Wait for pod to be ready by finding it with the label selector
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("resim.io/parentID=%s,resim.io/role=customer", debugExperience.BatchID),
	}

	namespace := "dev-env-pr-1560-e2e-resim-ai"
	var pod *v1.Pod
	for i := 0; i < 24; i++ { // 24 * 5 = 2 minutes
		pods, err := clientSet.CoreV1().Pods(namespace).List(ctx, listOptions)
		if err != nil {
			panic(err)
		}
		if len(pods.Items) > 0 {
			pod = &pods.Items[0]
			if pod.Status.Phase == v1.PodRunning {
				break
			}
		}
		time.Sleep(5 * time.Second)
	}

	if pod == nil {
		log.Fatal("Could not find running pod with matching label")
	}

	req := clientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name). // Use the found pod name instead of debugExperience.PodName
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&v1.PodExecOptions{
		Container: "",
		Command:   strings.Fields(command),
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
		// Env:       []string{"TERM=xterm", "PS1=\\$ "}, // Add basic environment settings
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(&restConfig, "POST", req.URL())
	if err != nil {
		panic(err)
	}

	t := term.TTY{
		In:  os.Stdin,
		Out: os.Stdout,
		Raw: true,
	}
	if err := t.Safe(term.SafeFunc(func() error { return nil })); err != nil {
		panic(err)
	}

	// use dockerterm.StdStreams() to get the right I/O handles on Windows
	overrideStreams := dockerterm.StdStreams

	stdin, stdout, _ := overrideStreams()
	t.In = stdin
	t.Out = stdout
	sizeQueue := t.MonitorSize(t.GetSize())

	err = exec.StreamWithContext(
		context.TODO(),
		remotecommand.StreamOptions{
			Stdin:             t.In,
			Stdout:            t.Out,
			Stderr:            t.Out,
			Tty:               true,
			TerminalSizeQueue: sizeQueue,
		})
	if err != nil {
		panic(err)
	}
}
