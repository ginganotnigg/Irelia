package handler

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
	pb "irelia/api"
	repo "irelia/internal/repo"
	sv "irelia/internal/service"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IreliaService implements the InterviewService gRPC interface for Frontend to Irelia communication
type IreliaService struct {
	pb.UnimplementedInterviewServiceServer
	dariusClient  sv.DariusClient
	karmaClient   sv.KarmaClient
	interviewRepo repo.SQLInterviewRepository
	logger        *zap.Logger
}

// NewIreliaService creates a new gRPC service for Frontend to Irelia communication
func NewIreliaService(dariusClient sv.DariusClient, karmaClient sv.KarmaClient, repo repo.SQLInterviewRepository, logger *zap.Logger) *IreliaService {
	return &IreliaService{
		dariusClient:  dariusClient,
		karmaClient:   karmaClient,
		interviewRepo: repo,
		logger:        logger,
	}
}

func (s *IreliaService) generateIntroQuestion() string {
	questions := []string{
		"Please provide a brief overview of your professional background and key qualifications.",
		"Reflecting on your professional journey, what is one area you are actively working to develop and why?",
		"What core strengths do you believe you bring to a role like this, and how have you demonstrated them in the past?",

	}

	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())
	randomIndex := rand.Intn(len(questions))

	return questions[randomIndex]
}

// Store the position-specific questions in a slice
func (s *IreliaService) generatePositionSpecificQuestions(position string) string {
	questions := []string{
		fmt.Sprintf("Could you describe some of your most engaging projects within %s?", position),
        fmt.Sprintf("What relevant experiences do you possess in the field of %s?", position),
		fmt.Sprintf("In your opinion, what are the primary challenges currently facing professionals in %s?", position),
        fmt.Sprintf("Could you share your journey into the field of %s?", position),
        fmt.Sprintf("What aspects of working within %s do you find particularly fulfilling?", position),
        fmt.Sprintf("What emerging trends within %s are you most enthusiastic about and why?", position),
        fmt.Sprintf("What are some prevalent misconceptions surrounding the profession of %s?", position),
        fmt.Sprintf("Can you elaborate on any professional development initiatives you've undertaken within %s?", position),
        fmt.Sprintf("What key advice would you offer to an individual beginning their career in %s?", position),
        fmt.Sprintf("What are your aspirations for your career trajectory within %s?", position),
        fmt.Sprintf("In your perspective, what are the paramount skills required for success in %s?", position),
	}
	rand.Seed(time.Now().UnixNano())
	randomIndex := rand.Intn(len(questions))

	return questions[randomIndex]
}

// Generate a unique interview ID
func (s *IreliaService) generateInterviewId() (string, error) {
	for {
		interviewID := uuid.New().String()
		_, err := s.interviewRepo.GetInterview(interviewID)
		if err == sql.ErrNoRows {
			return interviewID, nil
		}
		if err != nil {
			s.logger.Error("Failed to check interview ID uniqueness", zap.Error(err))
			return "", status.Errorf(codes.Internal, "Failed to generate unique interview ID: %v", err)
		}
	}
}

// Save questions to the database
func (s *IreliaService) saveQuestions(interviewID string, questions []string) error {
	for i, content := range questions {
		question := &pb.Question{
			Index:       int32(i + 1),
			InterviewId: interviewID,
			Content:     content,
			Status:      "new",
		}
		if err := s.interviewRepo.SaveQuestion(question); err != nil {
			s.logger.Error("Failed to save question", zap.Error(err))
			return fmt.Errorf("failed to save question: %v", err)
		}
	}
	return nil
}

// Generates the next question using Darius and saves it in the database
func (s *IreliaService) generateNextQuestion(interviewID string, submissions []*pb.QaPair, remainingQuestions int32, questionIndex int32) (*pb.Question, error) {
	// Retrieve the interview context
	interviewContext, err := s.interviewRepo.GetInterviewContext(interviewID)
	if err != nil {
		s.logger.Error("Failed to retrieve interview context", zap.String("interviewId", interviewID), zap.Error(err))
		return nil, fmt.Errorf("failed to retrieve interview context: %v", err)
	}

	// Provide default values for empty fields in the context
	if interviewContext.Position == "" {
		interviewContext.Position = "General"
	}
	if interviewContext.Experience == "" {
		interviewContext.Experience = "General"
	}
	if interviewContext.Language == "" {
		interviewContext.Language = "English"
	}
	if len(interviewContext.Skills) == 0 {
		interviewContext.Skills = map[string]string{"English skills": ""}
	}

	skills := make([]string, 0, len(interviewContext.Skills))
    for skill := range interviewContext.Skills {
        skills = append(skills, skill)
    }

	// Prepare the Darius request
	dariusReq := &pb.NextQuestionRequest{
		InterviewId:        interviewID,
		Submissions:        submissions,
		RemainingQuestions: remainingQuestions,
		Context: &pb.Context{
			Position:     interviewContext.Position,
			Experience:   interviewContext.Experience,
			Language:     interviewContext.Language,
			Skills:       skills,
			MaxQuestions: interviewContext.MaxQuestions,
			SkipCode:     interviewContext.SkipCode,
		},
	}

	var dariusResp *pb.NextQuestionResponse

	dariusResp, err = s.callDariusForGenerate(context.Background(), dariusReq)
	if err != nil {
		s.logger.Error("Failed to generate questions", zap.String("interviewId", interviewID))
		return nil, fmt.Errorf("failed to generate questions: %v", err)
	}

	if len(dariusResp.Questions) == 0 {
		s.logger.Error("No questions generated by Darius", zap.String("interviewId", interviewID))
		return nil, fmt.Errorf("no next question generated")
	}

	// Save the generated question(s) in the database
	for i, content := range dariusResp.Questions {
		question := &pb.Question{
			Index:       questionIndex + int32(i),
			InterviewId: interviewID,
			Content:     content,
			Status:      "new",
		}
		if err := s.interviewRepo.SaveQuestion(question); err != nil {
			s.logger.Error("Failed to save generated question", zap.String("interviewId", interviewID), zap.Int32("questionIndex", question.Index), zap.Error(err))
			return nil, fmt.Errorf("failed to save generated question: %v", err)
		}
	}

	// Return the first generated question
	return &pb.Question{
		Index:       questionIndex,
		InterviewId: interviewID,
		Content:     dariusResp.Questions[0],
		Status:      "new",
	}, nil
}

// Extract the substring after the last dot or dash in a string
func substringAfterLastDotOrDash(s string) string {
	lastDot := strings.LastIndex(s, ".")
	lastDash := strings.LastIndex(s, "-")

	if lastDot > lastDash {
		if lastDot == -1 {
			return ""
		}
		return s[lastDot+1:]
	} else if lastDash > lastDot {
		if lastDash == -1 {
			return ""
		}
		return s[lastDash+1:]
	} else {
		return "" // Neither '.' nor '-' found, or they are at the same experience (unlikely)
	}
}

// Add transitions to the question content
func (s *IreliaService) addTransitionsToQuestion(question *pb.Question, voiceID string) string {
	responseTokens := []string{
		"I see.", "That sounds good.", "Interesting.", "Got it.", "Alright.", "Understood.",
	}
	transitionSentences := []string{
		"Now, let's move on to the next question.", "Let's proceed to the next question.", "Moving on to the next question.",
		"Next question coming up.", "Here's the next question.",
	}

	rand.Seed(time.Now().UnixNano())
	if question.Index == 1 {
		// Add an intro for the first question
		intro := fmt.Sprintf("Thanks for joining this interview session today. I'm %s, nice to meet you. To begin with, let me ask you some questions.", substringAfterLastDotOrDash(voiceID))
		fullString := intro + " " + question.Content
		return fullString
	}
	responseToken := responseTokens[rand.Intn(len(responseTokens))]
	transition := transitionSentences[rand.Intn(len(transitionSentences))]
	fullString := responseToken + " " + transition + " " + question.Content
	return fullString
}

// Prepare lip-sync data for a question synchronously
func (s *IreliaService) prepareLipSync(interviewID string, question *pb.Question, voiceID string, speed int32, isOutro bool) error {
	if question == nil {
		s.logger.Error("Question is nil, cannot prepare lip sync", zap.String("interviewId", interviewID))
		return fmt.Errorf("question is nil, cannot prepare lip sync")
	}

	s.logger.Info("Preparing lip sync", zap.String("interviewId", interviewID), zap.String("content", question.Content))

	var fullString string
	if isOutro {
		fullString = question.Content
	} else {
		fullString = s.addTransitionsToQuestion(question, voiceID)
	}

	karmaReq := &pb.LipSyncRequest{
		InterviewId: interviewID,
		Content:     fullString,
		VoiceId:     voiceID,
		Speed:       speed,
	}

	karmaResp, err := s.callKarma(context.Background(), karmaReq)
	if err != nil {
		return fmt.Errorf("failed to generate lip sync: %v", err)
	}

	question.Audio = karmaResp.Audio
	question.Lipsync = karmaResp.Lipsync

	if isOutro {
		return nil
	}

	if err := s.interviewRepo.SaveQuestion(question); err != nil {
		s.logger.Error("Failed to save question's lip-sync data", zap.Error(err))
		return nil
	}

	return nil
}

// Prepare question data asynchronously
func (s *IreliaService) asyncPrepareQuestion(interviewID string, nextQuestionID int32, interview *pb.Interview) {
	exists, err := s.interviewRepo.QuestionExists(interviewID, nextQuestionID)
	if err != nil {
		s.logger.Error("Failed to check if question exists", zap.Error(err))
		return
	}

	if exists {
		s.logger.Info("Question already exists, skipping generation", zap.Int32("questionID", nextQuestionID))
	} else {
		submissions, err := s.interviewRepo.GetQaPair(interviewID)
		if err != nil {
			s.logger.Error("Failed to retrieve submissions for Darius", zap.Error(err))
			return
		}

		_, err = s.generateNextQuestion(interviewID, submissions, interview.MaxQuestions-nextQuestionID+1, nextQuestionID)
		if err != nil {
			s.logger.Error("Failed to generate next question", zap.Error(err))
			return
		}
	}

	nextQuestion, err := s.interviewRepo.GetQuestion(interviewID, nextQuestionID)
	if err != nil {
		s.logger.Error("Failed to retrieve next question", zap.Error(err))
		return
	}

	if nextQuestion != nil {
		// Prepare lip-sync data for the next question
		if err := s.prepareLipSync(interviewID, nextQuestion, interview.VoiceId, interview.Speed, false); err != nil {
			s.logger.Error("Failed to prepare lip sync for the next question", zap.Error(err))
		}
	}
}

// StartInterview handles the creation of a new interview
func (s *IreliaService) StartInterview(ctx context.Context, req *pb.StartInterviewRequest) (*pb.StartInterviewResponse, error) {
	s.logger.Info("Starting new interview", zap.String("experience", req.Experience))

	// Generate a unique interview ID
	interviewID, err := s.generateInterviewId()
	if err != nil {
		return nil, err
	}

	skills := make(map[string]string)
    for _, skill := range req.Skills {
        skills[skill] = ""
    }

	// Create and save the interview
	interview := &pb.Interview{
		Id:                 interviewID,
		Position:           req.Position,
		Experience:         req.Experience,
		Skills:             skills,
		Language:           req.Language,
		VoiceId:            req.Models,
		Speed:              req.Speed,
		MaxQuestions:       req.MaxQuestions,
		RemainingQuestions: req.MaxQuestions,
		SkipCode:           req.SkipCode,
		Status:             "InProgress",
	}
	if err := s.interviewRepo.SaveInterview(interview); err != nil {
		s.logger.Error("Failed to save interview", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to save interview: %v", err)
	}

	// Prepare questions
	questions := []string{}
	introQuestion := s.generateIntroQuestion()
	if !req.SkipIntro {
		questions = append(questions, introQuestion)
	}

	fieldQuestion := s.generatePositionSpecificQuestions(req.Position)
	questions = append(questions, fieldQuestion)

	if err := s.saveQuestions(interviewID, questions); err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	// Retrieve the first question
	firstQuestion, err := s.interviewRepo.GetQuestion(interviewID, 1)
	if err != nil {
		s.logger.Error("Failed to retrieve first question", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to retrieve first question: %v", err)
	}

	// Prepare lip-sync data for the first question synchronously
	if err := s.prepareLipSync(interviewID, firstQuestion, interview.VoiceId, interview.Speed, false); err != nil {
		s.logger.Error("Failed to prepare lip sync for the first question", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to prepare lip sync for the first question: %v", err)
	}

	// Prepare additional questions based on configuration
	questionsToPrepare := viper.GetInt("questions_to_prepare")
	for i := 1; i <= questionsToPrepare; i++ {
		nextQuestionID := int32(i + 1)
		if nextQuestionID > interview.MaxQuestions {
			break
		}

		go s.asyncPrepareQuestion(interviewID, nextQuestionID, interview)
	}

	// Return the response with the first question
	return &pb.StartInterviewResponse{
		InterviewId: interviewID,
	}, nil
}

// SubmitAnswer handles the submission of an answer for a question
func (s *IreliaService) SubmitAnswer(ctx context.Context, req *pb.SubmitAnswerRequest) (*pb.SubmitAnswerResponse, error) {
	s.logger.Info("Submitting answer", zap.String("interviewId", req.InterviewId))

	// Retrieve the interview
	_, err := s.interviewRepo.GetInterview(req.InterviewId)
	if err != nil {
		s.logger.Error("Interview not found", zap.String("interviewId", req.InterviewId), zap.Error(err))
		return nil, status.Errorf(codes.NotFound, "Interview not found: %v", err)
	}

	// Save the submitted answer
	answer := &pb.AnswerResult{
		Index:       req.Index,
		Answer:      req.Answer,
		RecordProof: req.RecordProof,
		Status:      "answered",
	}
	if err := s.interviewRepo.SaveAnswer(req.InterviewId, answer); err != nil {
		s.logger.Error("Failed to save answer", zap.Error(err))
		return &pb.SubmitAnswerResponse{Status: "Failed to save answer"}, nil
	}

	return &pb.SubmitAnswerResponse{Status: "Answer submitted successfully"}, nil

}

// GetNextQuestion retrieves the next question for an interview
func (s *IreliaService) GetNextQuestion(ctx context.Context, req *pb.QuestionRequest) (*pb.QuestionResponse, error) {
	s.logger.Info("Retrieving next question", zap.String("interviewId", req.InterviewId), zap.Int32("index", req.QuestionIndex))

	// Retrieve the interview
	interview, err := s.interviewRepo.GetInterview(req.InterviewId)
	if err != nil {
		s.logger.Error("Interview not found", zap.String("interviewId", req.InterviewId), zap.Error(err))
		return nil, status.Errorf(codes.NotFound, "Interview not found: %v", err)
	}

	if req.QuestionIndex > interview.MaxQuestions {
		return nil, status.Errorf(codes.InvalidArgument, "Question index out of range: %d", req.QuestionIndex)
	}

	// Retrieve the next question using the provided index
	question, err := s.interviewRepo.GetQuestion(req.InterviewId, req.QuestionIndex)
	if err != nil {
		if err == sql.ErrNoRows {
			s.logger.Error("Next question not found", zap.String("interviewId", req.InterviewId), zap.Int32("index", req.QuestionIndex), zap.Error(err))
			return nil, status.Errorf(codes.NotFound, "Next question not found: %v", err)
		}
		s.logger.Error("Failed to retrieve next question", zap.String("interviewId", req.InterviewId), zap.Int32("index", req.QuestionIndex), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to retrieve next question: %v", err)
	}

	// Prepare additional questions based on configuration
	questionsToPrepare := viper.GetInt("questions_to_prepare")
	for i := 1; i <= questionsToPrepare; i++ {
		nextQuestionID := req.QuestionIndex + int32(i)
		if nextQuestionID > interview.MaxQuestions {
			break
		}

		go s.asyncPrepareQuestion(req.InterviewId, nextQuestionID, interview)
	}

	// Determine if this is the last question
	isLastQuestion := req.QuestionIndex == interview.MaxQuestions

	// Return the next question
	return &pb.QuestionResponse{
		QuestionId:     question.Index,
		Content:        question.Content,
		Audio:          question.Audio,
		Lipsync:        question.Lipsync,
		IsLastQuestion: isLastQuestion,
	}, nil
}

// SubmitInterview handles the submission of the entire interview
func (s *IreliaService) SubmitInterview(ctx context.Context, req *pb.SubmitInterviewRequest) (*pb.SubmitInterviewResponse, error) {
	s.logger.Info("Submitting interview", zap.String("interviewId", req.InterviewId))

	// Retrieve the interview
	interview, err := s.interviewRepo.GetInterview(req.InterviewId)
	if err != nil {
		s.logger.Error("Interview not found", zap.String("interviewId", req.InterviewId), zap.Error(err))
		return nil, status.Errorf(codes.NotFound, "Interview not found: %v", err)
	}

	// Get all questions' AnswerData in interview
	answers, err := s.interviewRepo.GetAnswers(req.InterviewId)
	if err != nil {
		s.logger.Error("Failed to retrieve answers", zap.Error(err))
	}

	// Save the interview status
	interview.Status = "Pending"
	if err := s.interviewRepo.SaveInterview(interview); err != nil {
		s.logger.Error("Failed to save interview status", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to save interview status: %v", err)
	}

	// Get submissions from answers
	submissionsForDarius := make([]*pb.AnswerData, len(answers))
	// submissionForMorgana := make([]*pb.AnswerData, len(answers))
	for i, answer := range answers {
		submissionsForDarius[i] = &pb.AnswerData{
			Index:       answer.Index,
			Question:    answer.Content,
			Answer:      answer.Answer,
			RecordProof: "",
		}
		// submissionsForMorgana[i] = &pb.AnswerData{
		// 	Index:       answer.Index,
		// 	Question:    "",
		// 	Answer:      answer.Answer,
		// 	RecordProof: answer.RecordProof,
		// }
	}

	// Get ScoreInterviewRequest
	dariusReq := &pb.ScoreInterviewRequest{
		InterviewId: interview.Id,
		Submissions: submissionsForDarius,
	}
	// morganaReq := &pb.ScoreInterviewRequest{
	// 	InterviewId: interview.Id,
	// 	Submissions: submissionsForDarius,
	// }

	outro := &pb.Question{
		Content: "You have successfully submitted the interview. You can check out the results in a few minutes. See you in another interview session!",
	}

	// Prepare lip-sync data for the outro
	if err := s.prepareLipSync(req.InterviewId, outro, interview.VoiceId, interview.Speed, true); err != nil {
		s.logger.Error("Failed to prepare lip sync for the outro", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to prepare lip sync for the outro: %v", err)
	}

	go func() {
		dariusResp, err := s.callDariusForScore(context.Background(), dariusReq)
		if err != nil {
			s.logger.Error("Failed to score by Darius", zap.Error(err))
			return
		}
		// morganaResp, err := s.callMorgana(context.Background(), morganaReq)
		// if err != nil {
		// 	s.logger.Error("Failed to score by Morgana", zap.Error(err))
		// 	return
		// }

		// Update the database with scoring results
		for _, submission := range dariusResp.Submissions {
			answer := &pb.AnswerResult{
				Index:   submission.Index,
				Comment: submission.Comment,
				Score:   submission.Score,
			}
			if err := s.interviewRepo.SaveAnswer(req.InterviewId, answer); err != nil {
				s.logger.Error("Failed to save scoring result", zap.Error(err))
				return
			}
		}

		// Update the interview with feedback and total score
		for skill, score := range dariusResp.Skills {
			interview.Skills[skill] = score
		}
		interview.TotalScore = dariusResp.TotalScore
		interview.PositiveFeedback = dariusResp.PositiveFeedback
		interview.ActionableFeedback = dariusResp.ActionableFeedback
		interview.FinalComment = dariusResp.FinalComment
		interview.Status = "Completed"

		if err := s.interviewRepo.SaveInterview(interview); err != nil {
			s.logger.Error("Failed to save interview feedback", zap.Error(err))
			return
		}
	}()

	return &pb.SubmitInterviewResponse{
		Outro: &pb.LipSyncResponse{
			Audio:   outro.Audio,
			Lipsync: outro.Lipsync,
		},
	}, nil
}

// GetInterview retrieves the details of an interview
func (s *IreliaService) GetInterview(ctx context.Context, req *pb.GetInterviewRequest) (*pb.GetInterviewResponse, error) {
	s.logger.Info("Retrieving interview", zap.String("interviewId", req.InterviewId))

	// Retrieve the interview from the repository
	interview, err := s.interviewRepo.GetInterview(req.InterviewId)
	if err != nil {
		if err == sql.ErrNoRows {
			s.logger.Error("Interview not found", zap.String("interviewId", req.InterviewId), zap.Error(err))
			return nil, status.Errorf(codes.NotFound, "Interview not found: %v", err)
		}
		s.logger.Error("Failed to retrieve interview", zap.String("interviewId", req.InterviewId), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to retrieve interview: %v", err)
	}

	// Retrieve the answers
	answers, err := s.interviewRepo.GetAnswers(req.InterviewId)
	if err != nil {
		s.logger.Error("Failed to retrieve answers", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to retrieve answers: %v", err)
	}

	// Prepare the response
	response := &pb.GetInterviewResponse{
		InterviewId:        interview.Id,
		TotalScore:         interview.TotalScore,
		PositiveFeedback:   interview.PositiveFeedback,
		ActionableFeedback: interview.ActionableFeedback,
		FinalComment:       interview.FinalComment,
		Submissions:        answers,
	}

	return response, nil
}

// GetInterviewHistory retrieves the history of interviews
func (s *IreliaService) GetInterviewHistory(ctx context.Context, req *pb.GetInterviewHistoryRequest) (*pb.GetInterviewHistoryResponse, error) {
	s.logger.Info("Retrieving interview history", zap.Int32("page", req.Page))

	// Define pagination parameters
	const perPage = 10
	offset := (req.Page - 1) * perPage

	// Query the repository for interview history
	rows, err := s.interviewRepo.GetInterviewHistory(offset, perPage)
	if err != nil {
		s.logger.Error("Failed to retrieve interview history", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to retrieve interview history: %v", err)
	}

	// Prepare the response
	history := make([]*pb.InterviewSummary, 0)
	for _, row := range rows {
		history = append(history, &pb.InterviewSummary{
			InterviewId: row.InterviewId,
			Position:    row.Position,
			Experience:  row.Experience,
			TotalScore:  row.TotalScore,
			CreatedAt:   row.CreatedAt,
		})
	}

	// Calculate total pages
	totalCount, err := s.interviewRepo.GetTotalInterviewCount()
	if err != nil {
		s.logger.Error("Failed to retrieve total interview count", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to retrieve total interview count: %v", err)
	}
	totalPages := (totalCount + perPage - 1) / perPage

	return &pb.GetInterviewHistoryResponse{
		Page:       req.Page,
		PerPage:    perPage,
		TotalPages: totalPages,
		Interviews: history,
	}, nil
}

func (s *IreliaService) callDariusForGenerate(ctx context.Context, req *pb.NextQuestionRequest) (*pb.NextQuestionResponse, error) {
	// Construct the payload to match the expected structure
	submissions := make([]map[string]interface{}, len(req.Submissions))
	for i, submission := range req.Submissions {
		submissions[i] = map[string]interface{}{
			"question": submission.Question,
			"answer":   submission.Answer,
		}
	}

	payload := map[string]interface{}{
		"context": map[string]interface{}{
			"position":     req.Context.Position,
			"experience":   req.Context.Experience,
			"language":     req.Context.Language,
			"skills":       req.Context.Skills,
			"maxQuestions": req.Context.MaxQuestions,
			"skipCode":     req.Context.SkipCode,
		},
		"submissions":        submissions,
		"remainingQuestions": req.RemainingQuestions,
	}

	return s.dariusClient.CallDariusForGenerate(ctx, payload)
}

func (s *IreliaService) callDariusForScore(ctx context.Context, req *pb.ScoreInterviewRequest) (*pb.ScoreInterviewResponse, error) {
	// Construct the payload to match the expected structure
	submissions := make([]map[string]interface{}, len(req.Submissions))
	for i, submission := range req.Submissions {
		submissions[i] = map[string]interface{}{
			"index":       int32(i + 1),
			"question":    submission.Question,
			"answer":      submission.Answer,
			"recordProof": "",
		}
	}

	payload := map[string]interface{}{
		"submissions": submissions,
	}
	return s.dariusClient.CallDariusForScore(ctx, payload)
}

func (s *IreliaService) callKarma(ctx context.Context, req *pb.LipSyncRequest) (*pb.LipSyncResponse, error) {
	payload := map[string]interface{}{
		"content": req.Content,
		"voiceId": req.VoiceId,
		"speed":   req.Speed,
	}
	return s.karmaClient.CallKarma(ctx, payload)
}

// Note: Check payload cua callDariusForGenerate
