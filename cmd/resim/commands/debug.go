package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"encoding/base64"

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
	debugBatchKey      = "batch"
)

func init() {
	debugCmd.Flags().String(debugProjectKey, "", "The name or ID of the project to associate with the debug session")
	debugCmd.MarkFlagRequired(debugProjectKey)
	debugCmd.Flags().String(debugBuildKey, "", "The ID of the build to debug")
	debugCmd.Flags().String(debugBatchKey, "", "The ID of the batch to debug")
	debugCmd.Flags().String(debugExperienceKey, "", "The experience to debug")
	debugCmd.MarkFlagRequired(debugExperienceKey)
	rootCmd.AddCommand(debugCmd)
}

func enableRawMode(projectID uuid.UUID, batchID uuid.UUID) (restore func(), err error) {
	fd := os.Stdin.Fd()
	state, err := dockerterm.SetRawTerminal(fd)
	if err != nil {
		return nil, err
	}
	return func() {
		_ = dockerterm.RestoreTerminal(fd, state)
		fmt.Println("Exiting Debug Session...")
		cancelDebugBatch(projectID, batchID)
	}, nil
}

func debug(ccmd *cobra.Command, args []string) {
	ctx := context.Background()
	projectID := getProjectID(Client, viper.GetString(debugProjectKey))

	// pool label
	poolLabels := []string{"resim:k8s"}

	buildIDString := viper.GetString(debugBuildKey)
	batchRef := viper.GetString(debugBatchKey)

	if buildIDString == "" && batchRef == "" {
		log.Fatal("Either a build ID or batch ID must be provided")
	}

	if buildIDString != "" && batchRef != "" {
		log.Fatal("Only one of build ID or batch ID must be provided")
	}

	buildID, err := uuid.Parse(buildIDString)
	if err != nil {
		log.Fatal("invalid build ID: ", err)
	}

	experienceID := getExperienceID(Client, projectID, viper.GetString(debugExperienceKey), true)

	body := api.DebugExperienceJSONRequestBody{
		PoolLabels: &poolLabels,
	}

	if batchRef != "" {
		batch := actualGetBatch(projectID, "", batchRef)
		body.BatchID = batch.BatchID
	}

	if buildID != uuid.Nil {
		body.BuildID = &buildID
	}

	response, err := Client.DebugExperienceWithResponse(ctx, projectID, experienceID, body)
	if err != nil {
		log.Fatal("unable to debug experience: ", err)
	}

	ValidateResponse(http.StatusCreated, "unable to debug experience", response.HTTPResponse, response.Body)

	debugExperience := response.JSON201

	fmt.Println("Batch ID:", debugExperience.BatchID)
	fmt.Println("Waiting for debug environment to be ready...")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		fmt.Println("")
		cancelDebugBatch(projectID, *debugExperience.BatchID)
		os.Exit(0)
	}()

	command := "sh"

	serverAddress := debugExperience.ClusterEndpoint
	token := debugExperience.ClusterToken

	caData, err := base64.StdEncoding.DecodeString(*debugExperience.ClusterCAData)
	if err != nil {
		log.Fatal("Failed to decode cluster CA data: ", err)
	}

	restConfig := rest.Config{
		Host:        *serverAddress,
		BearerToken: *token,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: caData,
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

	var pod *v1.Pod
	for range make([]struct{}, 24) { // 24 * 5 = 2 minutes
		pods, err := clientSet.CoreV1().Pods(*debugExperience.Namespace).List(ctx, listOptions)
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
		Name(pod.Name).
		Namespace(*debugExperience.Namespace).
		SubResource("exec")

	req.VersionedParams(&v1.PodExecOptions{
		Container: "",
		Command:   strings.Fields(command),
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(&restConfig, "POST", req.URL())
	if err != nil {
		panic(err)
	}

	t := term.TTY{
		In:  os.Stdin,
		Out: os.Stdout,
		Raw: true,
		// Parent: term.StdStreams(),
	}
	if err := t.Safe(term.SafeFunc(func() error { return nil })); err != nil {
		panic(err)
	}

	restore, err := enableRawMode(projectID, *debugExperience.BatchID)
	if err != nil {
		panic(err)
	}
	defer restore()

	stdin, stdout, _ := dockerterm.StdStreams()
	t.In = stdin
	t.Out = stdout
	sizeQueue := t.MonitorSize(t.GetSize())

	err = exec.StreamWithContext(
		context.TODO(),
		remotecommand.StreamOptions{
			Stdin:             t.In,
			Stdout:            t.Out,
			Stderr:            os.Stderr,
			Tty:               true,
			TerminalSizeQueue: sizeQueue,
		})
	if err != nil {
		fmt.Println("Error streaming: ", err)
	}
}

func cancelDebugBatch(projectID uuid.UUID, batchID uuid.UUID) {
	fmt.Println("Cancelling debug batch...")
	response, err := Client.CancelBatchWithResponse(context.TODO(), projectID, batchID)
	if err != nil {
		log.Fatal("unable to cancel batch: ", err)
	}

	ValidateResponse(http.StatusOK, "unable to cancel batch", response.HTTPResponse, response.Body)
}
