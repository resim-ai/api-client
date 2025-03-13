package commands

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	logsCmd = &cobra.Command{
		Use:     "logs",
		Short:   "logs contains commands for listing and downloading test logs.",
		Long:    ``,
		Aliases: []string{"log"},
	}

	listLogsCmd = &cobra.Command{
		Use:   "list",
		Short: "list - Lists the logs for a batch",
		Long:  ``,
		Run:   listLogs,
	}

	downloadLogsCmd = &cobra.Command{
		Use:     "download",
		Short:   "download - Downloads the logs for a batch",
		Long:    ``,
		Run:     downloadLogs,
		Aliases: []string{"fetch"},
	}
)

const (
	logProjectKey = "project"
	logBatchIDKey = "batch-id"
	logTestIDKey  = "test-id" // User-facing is test ID, internal is job id
	logOutputKey  = "output"
	logGithubKey  = "github"
)

func init() {
	listLogsCmd.Flags().String(logProjectKey, "", "The name or ID of the project to list logs with")
	listLogsCmd.MarkFlagRequired(logProjectKey)
	listLogsCmd.Flags().String(logBatchIDKey, "", "The UUID of the batch the logs are associated with")
	listLogsCmd.MarkFlagRequired(logBatchIDKey)
	listLogsCmd.Flags().String(logTestIDKey, "", "The UUID of the test in the batch to list logs for")
	listLogsCmd.MarkFlagRequired(logTestIDKey)
	listLogsCmd.Flags().SetNormalizeFunc(aliasProjectNameFunc)
	logsCmd.AddCommand(listLogsCmd)

	downloadLogsCmd.Flags().String(logProjectKey, "", "The name or ID of the project to list logs with")
	downloadLogsCmd.MarkFlagRequired(logProjectKey)
	downloadLogsCmd.Flags().String(logBatchIDKey, "", "The UUID of the batch the logs are associated with")
	downloadLogsCmd.MarkFlagRequired(logBatchIDKey)
	downloadLogsCmd.Flags().String(logTestIDKey, "", "The UUID of the test in the batch to list logs for")
	downloadLogsCmd.MarkFlagRequired(logTestIDKey)
	downloadLogsCmd.Flags().String(logOutputKey, "", "The directory to download the logs to, must be empty if it already exists")
	downloadLogsCmd.MarkFlagRequired(logOutputKey)
	downloadLogsCmd.Flags().Bool(logGithubKey, false, "Whether to output log messages in GitHub Actions friendly format")
	downloadLogsCmd.Flags().SetNormalizeFunc(aliasProjectNameFunc)
	logsCmd.AddCommand(downloadLogsCmd)

	rootCmd.AddCommand(logsCmd)
}

func listJobLogsForJob(projectID uuid.UUID, batchID uuid.UUID, testID uuid.UUID) ([]api.JobLog, error) {
	logs := []api.JobLog{}
	var pageToken *string = nil
	for {
		response, err := Client.ListJobLogsForJobWithResponse(context.Background(), projectID, batchID, testID, &api.ListJobLogsForJobParams{
			PageToken: pageToken,
			PageSize:  Ptr(100),
		})
		if err != nil {
			return nil, err
		}
		ValidateResponse(http.StatusOK, "unable to list logs", response.HTTPResponse, response.Body)
		if response.JSON200.Logs == nil {
			return nil, errors.New("log list in response is nil")
		}
		responseLogs := *response.JSON200.Logs
		logs = append(logs, responseLogs...)

		if response.JSON200.NextPageToken != nil && *response.JSON200.NextPageToken != "" {
			pageToken = response.JSON200.NextPageToken
		} else {
			break
		}
	}
	return logs, nil
}

func listLogs(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(logProjectKey))
	batchID, err := uuid.Parse(viper.GetString(logBatchIDKey))
	if err != nil || batchID == uuid.Nil {
		log.Fatal("unable to parse batch ID: ", err)
	}

	testID, err := uuid.Parse(viper.GetString(logTestIDKey))
	if err != nil || testID == uuid.Nil {
		log.Fatal("unable to parse test ID: ", err)
	}

	logs, err := listJobLogsForJob(projectID, batchID, testID)
	if err != nil {
		log.Fatal("unable to fetch logs: ", err)
	}
	OutputJson(logs)
}

func isFileZipped(f os.File) (bool, error) {
	f.Seek(0, io.SeekStart)
	info, err := os.Stat(f.Name())
	if err != nil {
		return false, errors.New("could not 'stat' file")
	}

	fileHead := make([]byte, 512)
	if info.Size() < 512 {
		fileHead = make([]byte, info.Size())
	}

	_, err = f.Read(fileHead)
	if err != nil {
		return false, errors.Join(errors.New("failed to read file to determine content type"), err)
	}

	thisFileType := http.DetectContentType(fileHead)
	return thisFileType == "application/zip", nil
}

func unzipFile(zippedFile os.File) error {
	zippedFilePath, err := filepath.Abs(zippedFile.Name())
	if err != nil {
		return errors.New("failed to read zip file")
	}

	r, err := zip.OpenReader(zippedFilePath)
	if err != nil {
		return errors.New("failed to read zip file")
	}
	var deferredErr error
	defer func() {
		if err := r.Close(); err != nil {
			deferredErr = errors.New("failed to close zip file")
		}
	}()

	dest := filepath.Dir(zippedFilePath)
	err = os.MkdirAll(dest, 0o777)
	if err != nil {
		return errors.New("unable to make destination directory for zip file")
	}
	err = os.Chmod(dest, 0o777)
	if err != nil {
		return errors.New("could not chmod destination directory")
	}

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		var deferredErr error
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				if deferredErr == nil {
					// this is the second deferred function (written first) so don't overwrite the error
					deferredErr = errors.New("failed to close unzipped file")
				}
			}
		}()

		path := filepath.Join(dest, f.Name)

		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			err := os.MkdirAll(path, f.Mode())
			if err != nil {
				return err
			}
		} else {
			err := os.MkdirAll(filepath.Dir(path), 0o777)
			if err != nil {
				return err
			}
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					deferredErr = errors.New("failed to close unzipped file")
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return deferredErr
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return errors.New("could not unzip file")
		}
	}
	return deferredErr
}

func downloadLogToFile(jobLog api.JobLog, file *os.File) error {
	resp, err := http.Get(*jobLog.LogOutputLocation)

	if err != nil {
		return errors.New("unable to download log: " + err.Error())
	}
	defer resp.Body.Close()

	bytesWritten, err := io.Copy(file, resp.Body)
	if err != nil {
		return errors.New("unable to write log file: " + err.Error())
	}
	if bytesWritten != *jobLog.FileSize {
		return fmt.Errorf("wrote %d bytes to %s but expected %d", bytesWritten, file.Name(), *jobLog.FileSize)
	}
	return nil
}

func downloadLogs(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(logProjectKey))
	batchID, err := uuid.Parse(viper.GetString(logBatchIDKey))
	if err != nil || batchID == uuid.Nil {
		log.Fatal("unable to parse batch ID: ", err)
	}

	testID, err := uuid.Parse(viper.GetString(logTestIDKey))
	if err != nil || testID == uuid.Nil {
		log.Fatal("unable to parse test ID: ", err)
	}

	outputDir := viper.GetString(logOutputKey)
	// if the directory exists, we need to check if it is empty
	// otherwise, we need to create it
	if files, err := os.ReadDir(outputDir); !os.IsNotExist(err) {
		if err != nil {
			log.Fatal("unable to read output directory: ", err)
		}
		if len(files) > 0 {
			log.Fatal("output directory is not empty: ", outputDir)
		}
	} else {
		os.MkdirAll(outputDir, 0755)
	}

	outputDir, err = filepath.Abs(outputDir)
	if err != nil {
		log.Fatal("unable to get absolute path for output directory: ", err)
	}

	s := NewSpinner(ccmd)
	s.Start("Getting list of logs...")

	logs, err := listJobLogsForJob(projectID, batchID, testID)
	if err != nil {
		log.Fatal("unable to fetch logs: ", err)
	}

	for _, jobLog := range logs {
		s.Update(fmt.Sprintf("Downloading %s...", *jobLog.FileName))

		filePath := filepath.Join(outputDir, *jobLog.FileName)
		out, err := os.Create(filePath)
		if err != nil {
			log.Fatal("unable to create log file: ", err)
		}

		err = downloadLogToFile(jobLog, out)
		if err != nil {
			log.Fatal("unable to download log: ", err)
		}

		if *jobLog.LogType == api.ARCHIVELOG {
			isZipped, err := isFileZipped(*out)
			if err != nil {
				log.Fatal("unable to determine if log file is zipped: ", err)
			}

			if isZipped {
				s.Update(fmt.Sprintf("Unzipping %s...", *jobLog.FileName))
				err = unzipFile(*out)
				if err != nil {
					log.Fatal("unable to unzip log file: ", err)
				}
				out.Close()
				os.Remove(filePath)
				continue
			}
		} else {
			out.Close()
		}
	}

	s.Stop(nil)
	fmt.Printf("Downloaded %d logs to %s\n", len(logs), outputDir)
}
