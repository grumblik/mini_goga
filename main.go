package main
import (
    "bufio"
    "fmt"
    "net/http"
    "time"
    "os"
    "log"
)

var debug_mode int8 = 0
var query_err int8 = 0
var string_url []string
var result []string
var temp_result []string

func health(w http.ResponseWriter, req *http.Request) {
    fmt.Fprintf(w, "health\n")
}

func metric(w http.ResponseWriter, req *http.Request) {
  for _, each_ln := range result {
    fmt.Fprintf(w, each_ln)
  }
}

func curl() {
//  start := time.Now()
  for i := 1; i > 0 ; i++ {
    for _, each_ln := range string_url {
      first := time.Now().UnixNano()

      resp, err := http.Get(each_ln)
      if err != nil {
        fmt.Println("Err:", err)
        fmt.Println("Resp:", resp)
        temp_result = append(temp_result, fmt.Sprint("mini_goga_time{url=\"", each_ln, "\",error_msg=\"", err,"\",error=\"1\"} 100000 \n" ))
      } else {
        defer resp.Body.Close()
        second := time.Now().UnixNano()
        diff := (second - first) / 1000000
        temp_result = append(temp_result, fmt.Sprint("mini_goga_time{url=\"", each_ln, "\",code=\"", resp.StatusCode, "\",error=\"", query_err, "\"} ", diff, "\n" ))
      }

      if debug_mode != 0 {
        fmt.Println("Response: ", resp, "\n")
      }
    }
    result = nil
    result = temp_result
    result = append(result, fmt.Sprint("mini_goga_cycle ", i, "\n" ))
    result = append(result, fmt.Sprint("mini_goga_url ", len(string_url), "\n" ))
    temp_result = nil
    time.Sleep(1 * time.Second)
  }
}

func main() {
  config := os.Getenv("CONFIG")

  file, err := os.Open(config)
  if err != nil {
    if debug_mode > 0 {
      log.Fatalf("failed to open ", config)
    }
  }

  scanner := bufio.NewScanner(file)

  scanner.Split(bufio.ScanLines)

  for scanner.Scan() {
    string_url = append(string_url, scanner.Text())
  }

  file.Close()

  go curl()

  http.HandleFunc("/health", health)
  http.HandleFunc("/metric", metric)
  http.ListenAndServe(":9190", nil)
}
