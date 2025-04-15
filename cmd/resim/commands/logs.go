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
	"slices"
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
		Short:   "logs contains commands for listing and downloading logs.",
		Long:    ``,
		Aliases: []string{"log"},
	}

	listLogsCmd = &cobra.Command{
		Use:   "list",
		Short: "list - Lists the logs for a batch or test",
		Long:  ``,
		Run:   listLogs,
	}

	downloadLogsCmd = &cobra.Command{
		Use:     "download",
		Short:   "download - Downloads the logs for a batch or test",
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
	logFilesKey   = "files"
)

func init() {
	listLogsCmd.Flags().String(logProjectKey, "", "The name or ID of the project to list logs with")
	listLogsCmd.MarkFlagRequired(logProjectKey)
	listLogsCmd.Flags().String(logBatchIDKey, "", "The UUID of the batch the logs are associated with")
	listLogsCmd.MarkFlagRequired(logBatchIDKey)
	listLogsCmd.Flags().String(logTestIDKey, "", "The UUID of the test in the batch to list logs for")
	listLogsCmd.Flags().SetNormalizeFunc(aliasProjectNameFunc)
	logsCmd.AddCommand(listLogsCmd)

	downloadLogsCmd.Flags().String(logProjectKey, "", "The name or ID of the project to list logs with")
	downloadLogsCmd.MarkFlagRequired(logProjectKey)
	downloadLogsCmd.Flags().String(logBatchIDKey, "", "The UUID of the batch the logs are associated with")
	downloadLogsCmd.MarkFlagRequired(logBatchIDKey)
	downloadLogsCmd.Flags().String(logTestIDKey, "", "The UUID of the test in the batch to list logs for")
	downloadLogsCmd.Flags().String(logOutputKey, "", "The directory to download the logs to")
	downloadLogsCmd.MarkFlagRequired(logOutputKey)
	downloadLogsCmd.Flags().Bool(logGithubKey, false, "Whether to output log messages in GitHub Actions friendly format")
	downloadLogsCmd.Flags().StringSlice(logFilesKey, []string{}, "The files to download from the logs, separated by commas. If not provided, all logs will be downloaded. If files are not all found, the command will exit with an error.")
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

func listBatchLogsForBatch(projectID uuid.UUID, batchID uuid.UUID) ([]api.BatchLog, error) {
	logs := []api.BatchLog{}
	var pageToken *string = nil
	for {
		response, err := Client.ListBatchLogsForBatchWithResponse(context.Background(), projectID, batchID, &api.ListBatchLogsForBatchParams{
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

	if viper.GetString(logTestIDKey) != "" {
		testID, err := uuid.Parse(viper.GetString(logTestIDKey))
		if err != nil || testID == uuid.Nil {
			log.Fatal("unable to parse test ID: ", err)
		}
		logs, err := listJobLogsForJob(projectID, batchID, testID)
		if err != nil {
			log.Fatal("unable to fetch tests logs: ", err)
		}
		OutputJson(logs)
	} else {
		logs, err := listBatchLogsForBatch(projectID, batchID)
		if err != nil {
			log.Fatal("unable to fetch batch logs: ", err)
		}
		OutputJson(logs)
	}
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

type DownloadableLog struct {
	FileName          string
	LogOutputLocation string
	FileSize          int64
	LogType           api.LogType
}

func DownloadableLogsFromJobLogs(jobLogs []api.JobLog) []DownloadableLog {
	downloadableLogs := []DownloadableLog{}
	for _, jobLog := range jobLogs {
		downloadableLogs = append(downloadableLogs, DownloadableLog{
			FileName:          *jobLog.FileName,
			LogOutputLocation: *jobLog.LogOutputLocation,
			FileSize:          *jobLog.FileSize,
			LogType:           *jobLog.LogType,
		})
	}
	return downloadableLogs
}

func DownloadableLogsFromBatchLogs(batchLogs []api.BatchLog) []DownloadableLog {
	downloadableLogs := []DownloadableLog{}
	for _, batchLog := range batchLogs {
		downloadableLogs = append(downloadableLogs, DownloadableLog{
			FileName:          *batchLog.FileName,
			LogOutputLocation: *batchLog.LogOutputLocation,
			FileSize:          *batchLog.FileSize,
			LogType:           *batchLog.LogType,
		})
	}
	return downloadableLogs
}

func filterLogs(downloadableLogs []DownloadableLog, fileNames []string) ([]DownloadableLog, error) {
	// filter the logs to only include the ones that match the file names
	// if not all files are present, return an error
	filteredLogs := []DownloadableLog{}
	for _, log := range downloadableLogs {
		if slices.Contains(fileNames, log.FileName) {
			filteredLogs = append(filteredLogs, log)
		}
	}
	if len(filteredLogs) != len(fileNames) {
		return nil, errors.New("not all expected logs were found")
	}
	return filteredLogs, nil
}

func downloadLogToFile(downloadableLog DownloadableLog, file *os.File) error {
	resp, err := http.Get(downloadableLog.LogOutputLocation)

	if err != nil {
		return errors.New("unable to download log: " + err.Error())
	}
	defer resp.Body.Close()

	bytesWritten, err := io.Copy(file, resp.Body)
	if err != nil {
		return errors.New("unable to write log file: " + err.Error())
	}
	if bytesWritten != downloadableLog.FileSize {
		return fmt.Errorf("wrote %d bytes to %s but expected %d", bytesWritten, file.Name(), downloadableLog.FileSize)
	}
	return nil
}

func downloadLogs(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(logProjectKey))

	outputDir := viper.GetString(logOutputKey)

	// we need to create the directory if it doesn't exist
	if _, err := os.ReadDir(outputDir); os.IsNotExist(err) {
		os.MkdirAll(outputDir, 0755)
	}

	outputDir, err := filepath.Abs(outputDir)
	if err != nil {
		log.Fatal("unable to get absolute path for output directory: ", err)
	}

	batchID, err := uuid.Parse(viper.GetString(logBatchIDKey))
	if err != nil || batchID == uuid.Nil {
		log.Fatal("unable to parse batch ID: ", err)
	}

	s := NewSpinner(ccmd)
	s.Start("Getting list of logs...")

	var downloadableLogs []DownloadableLog
	if viper.GetString(logTestIDKey) != "" {
		testID, err := uuid.Parse(viper.GetString(logTestIDKey))
		if err != nil || testID == uuid.Nil {
			s.Fatal("unable to parse test ID: ", err)
		}
		jobLogs, err := listJobLogsForJob(projectID, batchID, testID)
		if err != nil {
			s.Fatal("unable to fetch logs: ", err)
		}
		downloadableLogs = DownloadableLogsFromJobLogs(jobLogs)
	} else {
		batchLogs, err := listBatchLogsForBatch(projectID, batchID)
		if err != nil {
			s.Fatal("unable to fetch logs: ", err)
		}
		downloadableLogs = DownloadableLogsFromBatchLogs(batchLogs)
	}

	// if the user provided a list of files to download, we need to filter the logs and make sure all files are found
	if len(viper.GetStringSlice(logFilesKey)) > 0 {
		downloadableLogs, err = filterLogs(downloadableLogs, viper.GetStringSlice(logFilesKey))
		if err != nil {
			s.Fatal("unable to download logs: ", err)
		}
	}

	for _, downloadableLog := range downloadableLogs {
		s.Update(fmt.Sprintf("Downloading %s...", downloadableLog.FileName))

		filePath := filepath.Join(outputDir, downloadableLog.FileName)
		out, err := os.Create(filePath)
		if err != nil {
			s.Fatal("unable to create log file: ", err)
		}

		err = downloadLogToFile(downloadableLog, out)
		if err != nil {
			s.Fatal("unable to download log: ", err)
		}

		if downloadableLog.LogType == api.ARCHIVELOG {
			isZipped, err := isFileZipped(*out)
			if err != nil {
				s.Fatal("unable to determine if log file is zipped: ", err)
			}

			if isZipped {
				s.Update(fmt.Sprintf("Unzipping %s...", downloadableLog.FileName))
				err = unzipFile(*out)
				if err != nil {
					s.Fatal("unable to unzip log file: ", err)
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
	fmt.Printf("Downloaded %d log(s) to %s\n", len(downloadableLogs), outputDir)
}
