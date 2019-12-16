package stress

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lock-free/gopcp"
	"github.com/logrusorgru/aurora"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"sync"
	"time"
)

type StressConfig struct {
	Apis []ApiConfig
}

type ApiConfig struct {
	Name string
	// request part
	Scheme  string // http, https
	Host    string // hostname
	Path    string //path
	Method  string // GET | POST | PUT | DELETE | HEAD | OPTION
	Headers map[string]string
	Body    interface{} // string || JSON object
	Timeout int         // seconds

	// qps controll part
	ReqPerSec     int // requests send per second
	Duration      int // last how long (seconds)
	MaxRunningReq int // max running requests

	// validation part
	Expect ApiExpect

	FailExit bool // if api testing fail, exit process
}

func GetRequestBody(apiConfig *ApiConfig) (string, error) {
	switch v := apiConfig.Body.(type) {
	case string:
		return v, nil
	default:
		bs, err := json.Marshal(apiConfig.Body)
		return string(bs), err
	}
}

const EXPECT_EQUAL = "equal"
const EXPECT_EQUAL_JSON = "equal_json"
const EXPECT_REG = "reg" // regular expression
const EXPECT_PCP = "pcp" //pcp expression

type ApiExpect struct {
	Status         []int
	BodyExpectType string
	BodyExp        interface{}
	LogBody        bool
}

func TestApi(apiConfig *ApiConfig) error {
	client := http.Client{
		Timeout: time.Duration(time.Duration(apiConfig.Timeout) * time.Second),
	}

	url := apiConfig.Scheme + "://" + apiConfig.Host + apiConfig.Path
	body, err := GetRequestBody(apiConfig)
	if err != nil {
		return err
	}
	request, err := http.NewRequest(apiConfig.Method, url, bytes.NewBufferString(body))
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

	// check body
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// check status code
	if !Contains(apiConfig.Expect.Status, resp.StatusCode) {
		return errors.New(fmt.Sprintf("Status check fail, http status code is %d, expected status are %v. Body is %s", resp.StatusCode, apiConfig.Expect.Status, string(bodyBytes)))
	}

	// TODO check header

	if apiConfig.Expect.LogBody {
		log.Println("[log body] " + string(bodyBytes))
	}

	return CheckBody(bodyBytes, apiConfig.Expect)
}

func CheckBody(bodyBytes []byte, expect ApiExpect) error {
	body := string(bodyBytes)

	// check body
	switch expect.BodyExpectType {
	case EXPECT_EQUAL:
		if body != expect.BodyExp {
			return fmt.Errorf("Body check fail, body is %s, expect bodyExp is %v", body, expect.BodyExp)
		}

	case EXPECT_EQUAL_JSON:
		var value interface{}
		err := json.Unmarshal(bodyBytes, &value)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(value, expect.BodyExp) {
			return fmt.Errorf("Body check fail, body is %s, expect bodyExp is %v", body, expect.BodyExp)
		}

	case EXPECT_REG:
		bodyExp, ok := expect.BodyExp.(string)
		if !ok {
			return errors.New("BodyExp should be string")
		}
		matched, err := regexp.Match(bodyExp, bodyBytes)
		if err != nil {
			return err
		}
		if !matched {
			return fmt.Errorf("Body check fail, body is %s, expect bodyExp is %v", body, expect.BodyExp)
		}

	case EXPECT_PCP:
		bodyExp, ok := expect.BodyExp.(string)
		if !ok {
			return errors.New("BodyExp should be string")
		}
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
		r, err := pcpServer.Execute(bodyExp, nil)
		if err != nil {
			return err
		}

		matched, ok := r.(bool)
		if !ok {
			return fmt.Errorf("Body check fail, result of pcp is not boolean.")
		}

		if !matched {
			return fmt.Errorf("Body check fail, body is %s, expect bodyExp is %v", body, expect.BodyExp)
		}
	}

	return nil
}

func StressTestingApi(apiConfig ApiConfig) {
	log.Println(aurora.Blue(fmt.Sprintf("[api stress start] %v", apiConfig)))

	coner := GetConer(apiConfig.MaxRunningReq)
	totalRun, errorCount, reqTotalTime := 0, 0, int64(0)
	var statMutex = &sync.Mutex{}

	t1 := time.Now().UnixNano() / int64(time.Millisecond)
	var wg sync.WaitGroup

	for i := 0; i < apiConfig.Duration; i++ {
		for j := 0; j < apiConfig.ReqPerSec; j++ {
			accepted := coner.Run(func() {
				reqStart := time.Now().UnixNano() / int64(time.Millisecond)
				err := TestApi(&apiConfig)
				if apiConfig.FailExit {
					panic(err)
				}
				reqEnd := time.Now().UnixNano() / int64(time.Millisecond)
				reqTime := reqEnd - reqStart

				// do some statistics
				statMutex.Lock()
				reqTotalTime += reqTime
				if err != nil {
					log.Println(aurora.Red(fmt.Sprintf("[Errored] %v", err)))
					errorCount++
				}
				statMutex.Unlock()

				wg.Done()
			})

			if accepted {
				wg.Add(1)
				totalRun++
			}
		}

		// sleep 1 second
		time.Sleep(1 * time.Second)
	}

	wg.Wait()
	t2 := time.Now().UnixNano() / int64(time.Millisecond)
	// display statistics
	// TODO average http response time
	log.Println(
		aurora.Yellow(
			fmt.Sprintf("[api stress result] name = %s totalRun = %d, errored = %d, totalTime = %d s, avgReqTime = %d ms",
				apiConfig.Name,
				totalRun,
				errorCount,
				(t2-t1)/1000,
				reqTotalTime/int64(totalRun)),
		),
	)
}

func StressTesting(stressConfig StressConfig, host string, scheme string, only string) {
	for _, apiConfig := range stressConfig.Apis {
		if only == "" || only == apiConfig.Name {
			if host != "" {
				apiConfig.Host = host
			}

			if scheme != "" {
				apiConfig.Scheme = scheme
			}

			StressTestingApi(apiConfig)
		}
	}
}
