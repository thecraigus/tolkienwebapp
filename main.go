package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/template"
)

var tpl *template.Template

var bearer = os.Getenv("TOKEN")

type chardata struct {
	Submissions url.Values
}

type MovieResults2 struct {
	Docs []struct {
		ID               string `json:"_id"`
		Name             string `json:"name"`
		RuntimeInMinutes int    `json:"runtimeInMinutes"`
		BudgetInMillions int    `json:"budgetInMillions"`
		// Define interface so field can hold values of any type - API returns both int and float for this
		BoxOfficeRevenueInMillions interface{} `json:"boxOfficeRevenueInMillions"`
		AcademyAwardNominations    int         `json:"academyAwardNominations"`
		AcademyAwardWins           int         `json:"academyAwardWins"`
		RottenTomatoesScore        int         `json:"rottenTomatoesScore"`
	} `json:"docs"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Page   int `json:"page"`
	Pages  int `json:"pages"`
}

type CharResults2 struct {
	Docs []struct {
		ID      string `json:"_id"`
		Height  string `json:"height"`
		Race    string `json:"race"`
		Gender  string `json:"gender"`
		Birth   string `json:"birth"`
		Spouse  string `json:"spouse"`
		Death   string `json:"death"`
		Realm   string `json:"realm"`
		Hair    string `json:"hair"`
		Name    string `json:"name"`
		WikiURL string `json:"wikiUrl"`
	} `json:"docs"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Page   int `json:"page"`
	Pages  int `json:"pages"`
}

type QuoteResults2 struct {
	Docs []struct {
		// ID        string `json:"_id"`
		Dialog    string `json:"dialog"`
		Movie     string `json:"movie"`
		MovieName string
		Character string `json:"character"`
		ID        string `json:"id"`
	} `json:"docs"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Page   int `json:"page"`
	Pages  int `json:"pages"`
}

func UpdateQuoteName(tv *CharTemplateVars, in int, mname string) {
	tv.QuoteResults2.Docs[in].MovieName = mname
}

type ArtefactResults []struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Text       string `json:"text"`
	LotrPageID string `json:"lotr_page_id"`
	Character  string `json:"character"`
	LotrURL    string `json:"lotr_url"`
}

type CharTemplateVars struct {
	CharResults2    CharResults2
	QuoteResults2   QuoteResults2
	ArtefactResults ArtefactResults
}

func init() {
	tpl = template.Must(template.ParseGlob("templates/*.gohtml"))

}

func sorry(w http.ResponseWriter, req *http.Request, emsg struct{ Msg string }) {
	err := tpl.ExecuteTemplate(w, "errorpage.gohtml", emsg)

	if err != nil {
		log.Fatal("Error With Sorry Page")
	}
}

func index(w http.ResponseWriter, req *http.Request) {
	err := tpl.ExecuteTemplate(w, "index.gohtml", nil)

	if err != nil {
		log.Fatal("Error With Index")
	}
}

func charsearch(w http.ResponseWriter, req *http.Request) {
	err := req.ParseForm()

	if err != nil {
		log.Fatal("error parsing data:", err)
	}
	chardata := chardata{
		req.Form,
	}

	charchan := make(chan *http.Response)
	quotechan := make(chan *http.Response)
	moviechan := make(chan *http.Response)

	go GetChar(chardata, charchan)

	charname := <-charchan

	if charname.StatusCode == http.StatusTooManyRequests {
		emsg := struct {
			Msg string
		}{
			Msg: "Our service is rate limited - please try again later",
		}
		sorry(w, req, emsg)
		return
	}

	CharTemplateVars := &CharTemplateVars{}
	CharTemplateVars.CharResults2 = ParseChar(charname)

	if CharTemplateVars.CharResults2.Total == 0 {
		emsg := struct {
			Msg string
		}{
			Msg: "Unable to find Character: " + chardata.Submissions.Get("charname"),
		}
		sorry(w, req, emsg)
		return
	}

	charid := CharTemplateVars.CharResults2.Docs[0].ID

	go GetQuotes(charid, quotechan)
	quoteresp := <-quotechan

	if err != nil {
		log.Fatal("Error Reading Resp:", err)
	}

	if charname.StatusCode != 200 {
		http.Error(w, fmt.Sprintf("Unable to find Char: %s", chardata.Submissions.Get("charname")), 404)
		return
	}

	CharTemplateVars.QuoteResults2 = ParseQuote(quoteresp, w, req)

	for _, v := range CharTemplateVars.QuoteResults2.Docs {
		go GetMovies(v.Movie, moviechan)

	}

	for k, _ := range CharTemplateVars.QuoteResults2.Docs {
		movieresp := <-moviechan
		// fmt.Println(k, v)
		y := ParseMovie(movieresp, w, req)
		MovieName := y.Docs[0].Name
		UpdateQuoteName(CharTemplateVars, k, MovieName)

	}

	err = tpl.ExecuteTemplate(w, "charesults.gohtml", CharTemplateVars)
	if err != nil {
		log.Fatal("Error With Charsearch", err)
	}
}

func ParseChar(charResp *http.Response) CharResults2 {
	var CharResult CharResults2
	body, err := io.ReadAll(charResp.Body)
	if err != nil {
		log.Fatal("Error Reading Resp:", err)
	}

	err = json.Unmarshal(body, &CharResult)
	if err != nil {
		log.Fatal("Error Paring Char Data: ", err)
	}
	return CharResult

}

func ParseQuote(quoteResp *http.Response, w http.ResponseWriter, req *http.Request) QuoteResults2 {
	var QuoteResult QuoteResults2
	qbody, err := io.ReadAll(quoteResp.Body)
	if err != nil {
		log.Fatal("Error Reading Resp:", err)
	}

	err = json.Unmarshal(qbody, &QuoteResult)
	if err != nil {
		emsg := struct {
			Msg string
		}{
			Msg: "Our service is rate limted 1",
		}
		sorry(w, req, emsg)
		// return

	}
	return QuoteResult

}

func ParseMovie(movieResp *http.Response, w http.ResponseWriter, req *http.Request) MovieResults2 {
	var MovieResults MovieResults2
	qbody, err := io.ReadAll(movieResp.Body)
	if err != nil {
		log.Fatal("Error Reading Resp:", err)
	}

	err = json.Unmarshal(qbody, &MovieResults)
	if err != nil {
		emsg := struct {
			Msg string
		}{
			Msg: "Our service is rate limted 1",
		}
		sorry(w, req, emsg)
		// return

	}
	return MovieResults
}

func ParseArtefact(artefactresp *http.Response) ArtefactResults {
	var ArtefactResult ArtefactResults
	qbody, err := io.ReadAll(artefactresp.Body)
	if err != nil {
		log.Fatal("Error Reading Resp:", err)
	}

	err = json.Unmarshal(qbody, &ArtefactResult)
	if err != nil {
		log.Fatal("Error Paring Artefact Data: ", err)

	}
	return ArtefactResult

}

// refactor this into  a generic get char data call - pass in url and  channel
func GetChar(chardata chardata, charchan chan *http.Response) {
	// escape whitespacw with url.QueryEscape
	char := url.QueryEscape(strings.Title(strings.ToLower(chardata.Submissions.Get("charname"))))
	url := "https://the-one-api.dev/v2/character?name=" + char
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", bearer)
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		log.Fatal("Error WIth Char Search", err)
	}
	charchan <- resp
}

func GetQuotes(charid string, quotechan chan *http.Response) {
	url := "https://the-one-api.dev/v2/character/" + charid + "/quote?limit=10"
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", bearer)
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		log.Fatal("Error With Quote Search", err)
	}
	// fmt.Println(resp)
	quotechan <- resp
}

func GetMovies(movieid string, moviechan chan *http.Response) {
	url := "https://the-one-api.dev/v2/movie/" + movieid
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", bearer)
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		log.Fatal("Error With Quote Search", err)
	}
	// fmt.Println(resp)
	moviechan <- resp
}

func GetArtefacts(chardata chardata, artefactchan chan *http.Response) {
	char := strings.Title(strings.ToLower(chardata.Submissions.Get("charname")))
	resp, err := http.Get("https://tolkien-api.herokuapp.com/Artefacts/by/" + char)
	if err != nil {
		log.Fatal("Error With Artefact Search", err)
	}

	artefactchan <- resp
}

func main() {

	http.HandleFunc("/", index)

	http.HandleFunc("/charsearch", charsearch)

	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("./css"))))

	http.ListenAndServe(":8080", nil)
}
