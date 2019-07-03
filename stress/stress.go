package stress

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lock-free/gopcp"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"sync"
	"time"
)

func TestApi(apiConfig ApiConfig) error {
	client := http.Client{
		Timeout: time.Duration(time.Duration(apiConfig.Timeout) * time.Second),
	}

	url := apiConfig.Scheme + "://" + apiConfig.Host + apiConfig.Path
	request, err := http.NewRequest(apiConfig.Method, url, bytes.NewBufferString(apiConfig.Body))
	if err != nil {
		return err
	}

	for k, v := range apiConfig.Headers {
		request.Header.Set(k, v)
	}

	resp, err := client.Do(request)
	if err != nil {
		return err
	}

	// check status code
	if !Contains(apiConfig.Expect.Status, resp.StatusCode) {
		return errors.New(fmt.Sprintf("Status check fail, http status code is %d, expected status are %v", resp.StatusCode, apiConfig.Expect.Status))
	}

	// check body
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return CheckBody(bodyBytes, apiConfig.Expect)
}

func CheckBody(bodyBytes []byte, expect ApiExpect) error {
	body := string(bodyBytes)

	// check body
	switch expect.BodyExpectType {
	case EXPECT_EQUAL:
		if body != expect.BodyExp {
			return errors.New(fmt.Sprintf("Body check fail, body is %s", body))
		}

	case EXPECT_REG:
		matched, err := regexp.Match(expect.BodyExp, bodyBytes)
		if err != nil {
			return err
		}
		if !matched {
			return errors.New(fmt.Sprintf("Body check fail, body is %s", body))
		}

	case EXPECT_PCP:
		sandbox := gopcp.GetSandbox(map[string]*gopcp.BoxFunc{
			// return http body as string
			"getJson": gopcp.ToSandboxFun(func(args []interface{}, attachment interface{}, pcpServer *gopcp.PcpServer) (interface{}, error) {
				// return body, nil
				var v interface{}
				err := json.Unmarshal(bodyBytes, &v)
				return v, err
			}),
		})
		pcpServer := gopcp.NewPcpServer(sandbox)
		r, err := pcpServer.Execute(expect.BodyExp, nil)
		if err != nil {
			return err
		}

		matched, ok := r.(bool)
		if !ok {
			return errors.New(fmt.Sprintf("Body check fail, result of pcp is not boolean."))
		}

		if !matched {
			return errors.New(fmt.Sprintf("Body check fail, body is %s", body))
		}
	}

	return nil
}

func StressTestingApi(apiConfig ApiConfig) {
	log.Printf("[api stress] %v", apiConfig)
	t1 := time.Now().UnixNano() / int64(time.Second)

	coner := GetConer(apiConfig.MaxRunningReq)
	totalRun, errorCount := 0, 0
	var errorCountMutex = &sync.Mutex{}

	for i := 0; i < apiConfig.Duration; i++ {
		for j := 0; j < apiConfig.ReqPerSec; j++ {
			accepted := coner.Run(func() {
				err := TestApi(apiConfig)
				if err != nil {
					log.Printf("[Errored] %v", err)
					errorCountMutex.Lock()
					errorCount++
					errorCountMutex.Unlock()
				}
			})

			if accepted {
				totalRun++
			}
		}

		// sleep 1 second
		time.Sleep(1 * time.Second)
	}

	t2 := time.Now().UnixNano() / int64(time.Second)
	// display statistics
	// TODO average http response time
	log.Printf("[api stress result] totalRun = %d, errored = %d, totalTime = %dseconds", totalRun, errorCount, t2-t1)
}

func StressTesting(stressConfig StressConfig, host string, scheme string) {
	for _, apiConfig := range stressConfig.Apis {
		if host != "" {
			apiConfig.Host = host
		}

		if scheme != "" {
			apiConfig.Scheme = scheme
		}

		StressTestingApi(apiConfig)
	}
}

type StressConfig struct {
	Apis []ApiConfig
}

type ApiConfig struct {
	// request part
	Scheme  string // http, https
	Host    string // hostname
	Path    string //path
	Method  string // GET | POST | PUT | DELETE | HEAD | OPTION
	Headers map[string]string
	Body    string
	Timeout int // seconds

	// qps controll part
	ReqPerSec     int // requests send per second
	Duration      int // last how long (seconds)
	MaxRunningReq int // max running requests

	// validation part
	Expect ApiExpect
}

const EXPECT_EQUAL = "equal"
const EXPECT_REG = "reg" // regular expression
const EXPECT_PCP = "pcp" //pcp expression

type ApiExpect struct {
	Status         []int
	BodyExpectType string
	BodyExp        string
}
