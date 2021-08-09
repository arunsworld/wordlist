package wordlist

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/arunsworld/wordlist/pkg/website"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

func SetupQuiz(ws *website.Website) {
	if err := ws.DB().AutoMigrate(&OngoingQuiz{}); err != nil {
		panic(err)
	}
	if err := ws.DB().AutoMigrate(&OngoingQuizQuestion{}); err != nil {
		panic(err)
	}
	if err := ws.DB().AutoMigrate(&CompletedQuiz{}); err != nil {
		panic(err)
	}
	if err := ws.DB().AutoMigrate(&IncorrectWord{}); err != nil {
		panic(err)
	}

	quizHTML, err := ws.WebsiteContent().ReadFile("web/html/quiz.html")
	if err != nil {
		panic(err)
	}

	quiz := ws.Router().PathPrefix("/quiz/").Methods("GET").Subrouter()
	quiz.Use(ws.EnsureAuthMiddleware(website.AuthMiddlewareConfig{}))
	quiz.HandleFunc("/{count_str}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		count, err := strconv.Atoi(vars["count_str"])
		if err != nil {
			log.Printf("error creating a new quiz: %v", err)
			http.Error(w, "error creating new quiz", http.StatusInternalServerError)
			return
		}
		if count > 50 {
			count = 50
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write(bytes.ReplaceAll(quizHTML, []byte("[[COUNT]]"), []byte(strconv.Itoa(count))))
	})

	quizAPIGET := ws.Router().PathPrefix("/quiz-api/").Methods("GET").Subrouter()
	quizAPIGET.Use(ws.EnsureAuthMiddleware(website.AuthMiddlewareConfig{IsForAPI: true}))

	quizAPIPOST := ws.Router().PathPrefix("/quiz-api/").Methods("POST").Subrouter()
	quizAPIPOST.Use(ws.EnsureAuthMiddleware(website.AuthMiddlewareConfig{IsForAPI: true}))

	quizAPIGET.HandleFunc("/new/{count_str}", func(w http.ResponseWriter, r *http.Request) {
		qs, err := newQuizSession(ws.DB())
		if err != nil {
			log.Printf("error creating new quiz session: %v", err)
			http.Error(w, "error creating new quiz", http.StatusInternalServerError)
			return
		}
		vars := mux.Vars(r)
		count, err := strconv.Atoi(vars["count_str"])
		if err != nil {
			log.Printf("error creating new quiz session - unable to parse count: %v", err)
			http.Error(w, "error creating new quiz", http.StatusInternalServerError)
			return
		}
		quiz, err := qs.newQuiz(count, ws.AuthenticatedUser(r))
		if err != nil {
			log.Printf("error creating new quiz from session: %v", err)
			http.Error(w, "error creating new quiz", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		if err := json.NewEncoder(w).Encode(quiz); err != nil {
			log.Printf("error creating new quiz session: %v", err)
			http.Error(w, "error creating new quiz", http.StatusInternalServerError)
			return
		}
	})

	quizAPIPOST.HandleFunc("/save/", func(w http.ResponseWriter, r *http.Request) {
		answers := Answers{}
		err := json.NewDecoder(r.Body).Decode(&answers)
		if err != nil {
			log.Printf("error decoding answers as JSON: %v", err)
			http.Error(w, "error saving quiz results", http.StatusInternalServerError)
			return
		}
		resp, err := processAnswers(ws.DB(), answers)
		if err != nil {
			log.Printf("error processing answers: %v", err)
			http.Error(w, "error saving quiz results", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("error saving quiz answers: %v", err)
			http.Error(w, "error saving quiz results", http.StatusInternalServerError)
			return
		}
	})

}

func newQuizSession(db *gorm.DB) (quizSession, error) {
	qs := quizSession{
		db:  db,
		rnd: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	result := db.Find(&qs.allWords)
	if result.Error != nil {
		return qs, result.Error
	}
	for _, w := range qs.allWords {
		qs.allMeaningsAndWords = append(qs.allMeaningsAndWords, w.Word, w.Meaning)
	}
	return qs, nil
}

type quizSession struct {
	db                  *gorm.DB
	rnd                 *rand.Rand
	allWords            []Word
	allMeaningsAndWords []string
}

func (qs quizSession) randomMeanings(count int, meaning string, word string) []string {
	result := make([]string, 0, count)
	result = append(result, meaning)
	ignoreWords := map[string]struct{}{meaning: {}, word: {}}
	for len(result) < count {
		w := qs.allMeaningsAndWords[qs.rnd.Intn(len(qs.allMeaningsAndWords))]
		_, ok := ignoreWords[w]
		if ok {
			continue
		}
		result = append(result, w)
		ignoreWords[w] = struct{}{}
	}
	rand.Shuffle(len(result), func(i, j int) {
		result[i], result[j] = result[j], result[i]
	})
	return result
}

func (qs quizSession) newQuiz(count int, user *website.User) (Quiz, error) {
	if user == nil {
		return Quiz{}, fmt.Errorf("user not found")
	}
	questions := make([]Question, 0, count)
	ongoingQuestions := make([]OngoingQuizQuestion, 0, count)
	ignoreWords := map[string]struct{}{}
	for len(questions) < count {
		w := qs.allWords[qs.rnd.Intn(len(qs.allWords))]
		_, ok := ignoreWords[w.Word]
		if ok {
			continue
		}
		questions = append(questions, Question{
			Word:    w.Word,
			Choices: qs.randomMeanings(5, w.Meaning, w.Word),
		})
		ongoingQuestions = append(ongoingQuestions, OngoingQuizQuestion{
			Word:    w.Word,
			Meaning: w.Meaning,
			WordID:  int(w.ID),
		})
		ignoreWords[w.Word] = struct{}{}
	}
	quiz := Quiz{
		Session:   uuid.NewString(),
		Questions: questions,
	}
	oq := OngoingQuiz{
		Session:              quiz.Session,
		OngoingQuizQuestions: ongoingQuestions,
		UserID:               user.ID,
	}
	result := qs.db.Create(&oq)
	if result.Error != nil {
		return Quiz{}, result.Error
	}
	return quiz, nil
}

func processAnswers(db *gorm.DB, answers Answers) (QuizSaveResponse, error) {
	ongoingQuiz := &OngoingQuiz{}
	result := db.Preload("OngoingQuizQuestions").First(&ongoingQuiz, "session = ?", answers.Session)
	if result.Error != nil {
		return QuizSaveResponse{}, result.Error
	}
	ongoingQuizWords := map[string]OngoingQuizQuestion{}
	for _, question := range ongoingQuiz.OngoingQuizQuestions {
		ongoingQuizWords[question.Word] = question
	}
	allWords := []Answer{}
	ia := IncorrectAnswers{}
	iws := []IncorrectWord{}
	for _, answer := range answers.Answers {
		oqq, ok := ongoingQuizWords[answer.Word]
		if !ok {
			return QuizSaveResponse{}, fmt.Errorf("%s was not found in the session but reported in answers", answer.Word)
		}
		if answer.Answer != oqq.Meaning {
			ia = append(ia, IncorrectAnswer{
				Word:    answer.Word,
				Meaning: oqq.Meaning,
				Chosen:  answer.Answer,
			})
			iws = append(iws, IncorrectWord{
				Session: ongoingQuiz.Session,
				WordID:  oqq.WordID,
			})
		}
		allWords = append(allWords, Answer{
			Word:   answer.Word,
			Answer: oqq.Meaning,
		})
	}
	completedQuiz := &CompletedQuiz{
		Session:            ongoingQuiz.Session,
		UserID:             ongoingQuiz.UserID,
		TakenAt:            ongoingQuiz.CreatedAt,
		TotalQuestions:     len(ongoingQuizWords),
		IncorrectQuestions: len(iws),
	}
	result = db.Create(completedQuiz)
	if result.Error != nil {
		log.Printf("WARNING: unable to save completed quiz: %v", result.Error)
	}
	if len(iws) > 0 {
		result = db.Create(iws)
		if result.Error != nil {
			log.Printf("WARNING: unable to save incorrect words: %v", result.Error)
		}
	}
	result = db.Where("ongoing_quiz_id = ?", ongoingQuiz.Session).Delete(OngoingQuizQuestion{})
	if result.Error != nil {
		log.Printf("WARNING: unable to delete ongoing quiz questions: %v", result.Error)
	}
	result = db.Delete(ongoingQuiz)
	if result.Error != nil {
		log.Printf("WARNING: unable to delete ongoing quiz: %v", result.Error)
	}
	duration := time.Since(ongoingQuiz.CreatedAt).Round(time.Second).String()
	return QuizSaveResponse{IncorrectAnswers: ia, AllWords: allWords, Time: duration}, nil
}

type OngoingQuiz struct {
	Session              string                `gorm:"primaryKey"`
	OngoingQuizQuestions []OngoingQuizQuestion `gorm:"constraint:OnDelete:CASCADE;"`
	UserID               uint
	CreatedAt            time.Time
}

type OngoingQuizQuestion struct {
	ID            uint `gorm:"primaryKey"`
	OngoingQuizID string
	Word          string
	Meaning       string
	WordID        int
}

type CompletedQuiz struct {
	Session            string `gorm:"primaryKey"`
	UserID             uint
	TakenAt            time.Time
	TotalQuestions     int
	IncorrectQuestions int
}

type IncorrectWord struct {
	ID      uint `gorm:"primaryKey"`
	Session string
	WordID  int
	Word    Word
}

// Javascript object
type Quiz struct {
	Session   string
	Questions []Question
}

type Question struct {
	Word    string
	Choices []string
}

type Answers struct {
	Session string
	Answers []Answer
}

type Answer struct {
	Word   string
	Answer string
}

type QuizSaveResponse struct {
	IncorrectAnswers IncorrectAnswers
	AllWords         []Answer
	Time             string // humanized time
}

type IncorrectAnswers []IncorrectAnswer

type IncorrectAnswer struct {
	Word    string
	Meaning string
	Chosen  string
}
