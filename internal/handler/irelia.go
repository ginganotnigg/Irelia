package handler

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	pb "irelia/api"
	repo "irelia/internal/repo"
	sv "irelia/internal/service"

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
			Status:      "active",
		}
		if err := s.interviewRepo.SaveQuestion(question); err != nil {
			s.logger.Error("Failed to save question", zap.Error(err))
			return fmt.Errorf("failed to save question: %v", err)
		}
	}
	return nil
}

// generateNextQuestion generates the next question using Darius and saves it in the database
func (s *IreliaService) generateNextQuestion(interviewID string, submissions []*pb.QaPair, remainingQuestions int32, questionIndex int32) (*pb.Question, error) {
	// Retrieve the interview context
	interviewContext, err := s.interviewRepo.GetInterviewContext(interviewID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve interview context: %v", err)
	}

	// Provide default values for empty fields in the context
	if interviewContext.Field == "" {
		interviewContext.Field = "General"
	}
	if interviewContext.Position == "" {
		interviewContext.Position = "Unknown"
	}
	if interviewContext.Language == "" {
		interviewContext.Language = "English"
	}
	if interviewContext.Level == "" {
		interviewContext.Level = "Intermediate"
	}

	// Prepare the Darius request
	dariusReq := &pb.NextQuestionRequest{
		InterviewId:        interviewID,
		Submissions:        submissions,
		RemainingQuestions: remainingQuestions,
		Context:            interviewContext,
	}

	// Call Darius to generate the next question
	dariusResp, err := s.CallDarius(context.Background(), dariusReq)
	if err != nil {
		return nil, fmt.Errorf("failed to generate next question: %v", err)
	}

	if len(dariusResp.Questions) == 0 {
		return nil, fmt.Errorf("no next question generated")
	}

	// Save the generated question(s) in the database
	for i, content := range dariusResp.Questions {
		question := &pb.Question{
			Index:       int32(questionIndex + int32(i)),
			InterviewId: interviewID,
			Content:     content,
			Status:      "active",
		}
		if err := s.interviewRepo.SaveQuestion(question); err != nil {
			return nil, fmt.Errorf("failed to save next question: %v", err)
		}
	}

	// Return the first generated question
	return &pb.Question{
		Index:       questionIndex,
		InterviewId: interviewID,
		Content:     dariusResp.Questions[0],
		Status:      "active",
	}, nil
}

// Prepare lip-sync data for a question synchronously
func (s *IreliaService) prepareLipSync(interviewID string, question *pb.Question, voiceID string, speed int32) error {
	karmaReq := &pb.LipSyncRequest{
		InterviewId: interviewID,
		Content:     question.Content,
		VoiceId:     voiceID,
		Speed:       speed,
	}
	s.logger.Info("Preparing lip sync", zap.String("interviewId", interviewID), zap.String("content", question.Content))

	karmaResp, err := s.CallKarma(context.Background(), karmaReq)
	if err != nil {
		return fmt.Errorf("failed to generate lip sync: %v", err)
	}

	question.Audio = karmaResp.Audio
	question.Lipsync = karmaResp.Lipsync

	if err := s.interviewRepo.SaveQuestion(question); err != nil {
		return fmt.Errorf("failed to save lip sync data: %v", err)
	}

	return nil
}

// StartInterview handles the creation of a new interview
func (s *IreliaService) StartInterview(ctx context.Context, req *pb.StartInterviewRequest) (*pb.StartInterviewResponse, error) {
	s.logger.Info("Starting new interview", zap.String("position", req.Position))

	// Generate a unique interview ID
	interviewID, err := s.generateInterviewId()
	if err != nil {
		return nil, err
	}

	// Create and save the interview
	interview := &pb.Interview{
		Id:                 interviewID,
		Field:              req.Field,
		Position:           req.Position,
		Language:           req.Language,
		VoiceId:            req.Models,
		Speed:              req.Speed,
		Level:              req.Level,
		MaxQuestions:       req.MaxQuestions,
		RemainingQuestions: req.MaxQuestions,
		Coding:             req.Coding,
		Completed:          false,
	}
	if err := s.interviewRepo.SaveInterview(interview); err != nil {
		s.logger.Error("Failed to save interview", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to save interview: %v", err)
	}

	// Prepare questions
	questions := []string{}
	if !req.SkipIntro {
		questions = append(questions, "Could you introduce yourself?", "What are your strengths and weaknesses?")
	}
	questions = append(questions, fmt.Sprintf("What are your favourite projects in %s?", req.Field))
	questions = append(questions, fmt.Sprintf("What are your experiences with %s?", req.Field))

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
	if err := s.prepareLipSync(interviewID, firstQuestion, interview.VoiceId, interview.Speed); err != nil {
		s.logger.Error("Failed to prepare lip sync for the first question", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to prepare lip sync for the first question: %v", err)
	}

	// Optionally prepare lip-sync data for the second question asynchronously
	if len(questions) > 1 {
		go func() {
			secondQuestion, err := s.interviewRepo.GetQuestion(interviewID, 2)
			if err == nil {
				if err := s.prepareLipSync(interviewID, secondQuestion, interview.VoiceId, interview.Speed); err != nil {
					s.logger.Error("Failed to prepare lip sync for the second question", zap.Error(err))
				}
			}
		}()

	}

	// Return the response with the first question
	return &pb.StartInterviewResponse{
		InterviewId: interviewID,
		FirstQuestion: &pb.QuestionData{
			Index:   firstQuestion.Index,
			Content: firstQuestion.Content,
			Audio:   firstQuestion.Audio,
			Lipsync: firstQuestion.Lipsync,
		},
	}, nil
}

// SubmitAnswer handles the submission of an answer for a question
func (s *IreliaService) SubmitAnswer(ctx context.Context, req *pb.SubmitAnswerRequest) (*pb.SubmitAnswerResponse, error) {
	s.logger.Info("Submitting answer", zap.String("interviewId", req.InterviewId))

	// Retrieve the interview
	interview, err := s.interviewRepo.GetInterview(req.InterviewId)
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
		return nil, status.Errorf(codes.Internal, "Failed to save answer: %v", err)
	}

	// Retrieve the next question
	nextQuestionID := req.Index + 1
	if nextQuestionID > interview.MaxQuestions {
		return &pb.SubmitAnswerResponse{
			Question: nil, // No more questions
		}, nil
	}

	nextQuestion, err := s.interviewRepo.GetQuestion(req.InterviewId, nextQuestionID)
	if err != nil {
		s.logger.Error("Failed to get next question", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to get next question: %v", err)
	}

	// Prepare the next-next question by calling Darius
	nextNextQuestionID := nextQuestionID + 1
	if nextNextQuestionID <= interview.MaxQuestions {
		exists, err := s.interviewRepo.QuestionExists(req.InterviewId, nextNextQuestionID)
		if err != nil {
			s.logger.Error("Failed to check if next-next question exists", zap.Error(err))
			return nil, status.Errorf(codes.Internal, "Failed to check if next-next question exists: %v", err)
		}

		if !exists {
			go func() {
				submissions, err := s.interviewRepo.GetQaPair(req.InterviewId)
				if err != nil {
					s.logger.Error("Failed to retrieve submissions for Darius", zap.Error(err))
					return
				}

				_, err = s.generateNextQuestion(req.InterviewId, submissions, interview.MaxQuestions-nextNextQuestionID+1, nextNextQuestionID)
				if err != nil {
					s.logger.Error("Failed to generate next-next question", zap.Error(err))
				}
			}()
		}

		// Prepare lip-sync data for the next-next question asynchronously
		go func() {
			var nextNextQuestion *pb.Question
			if nextNextQuestion, err = s.interviewRepo.GetQuestion(req.InterviewId, nextNextQuestionID); err != nil {
				s.logger.Error("Next-next question not found", zap.Error(err))
				return
			}

			if err := s.prepareLipSync(req.InterviewId, nextNextQuestion, interview.VoiceId, interview.Speed); err != nil {
				s.logger.Error("Failed to prepare lip sync for the next question", zap.Error(err))
			}
		}()
	}

	// Return the response with the next question
	return &pb.SubmitAnswerResponse{
		Question: &pb.QuestionData{
			Index:   nextQuestion.Index,
			Content: nextQuestion.Content,
			Audio:   nextQuestion.Audio,
			Lipsync: nextQuestion.Lipsync,
		},
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

	// Save the answers
	for _, answer := range req.History {
		answerResult := &pb.AnswerResult{
			Index:       answer.Index,
			Answer:      answer.Answer,
			RecordProof: answer.RecordProof,
			Status:      "answered",
		}
		if err := s.interviewRepo.SaveAnswer(req.InterviewId, answerResult); err != nil {
			s.logger.Error("Failed to save answer", zap.Error(err))
			return nil, status.Errorf(codes.Internal, "Failed to save answer: %v", err)
		}
	}

	// Mark the interview as completed
	interview.Completed = true

	// Save the updated interview
	if err := s.interviewRepo.SaveInterview(interview); err != nil {
		s.logger.Error("Failed to save interview", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to save interview: %v", err)
	}

	return &pb.SubmitInterviewResponse{
		Status: "201 Created",
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
		AreasOfImprovement: interview.AreasOfImprovement,
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
			Field:       row.Field,
			Position:    row.Position,
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

func (s *IreliaService) CallDarius(ctx context.Context, req *pb.NextQuestionRequest) (*pb.NextQuestionResponse, error) {
	payload := map[string]interface{}{
		"interview_id":        req.InterviewId,
		"submissions":         req.Submissions,
		"remaining_questions": req.RemainingQuestions,
		"context": map[string]interface{}{
			"field":         req.Context.Field,
			"position":      req.Context.Position,
			"language":      req.Context.Language,
			"level":         req.Context.Level,
			"max_questions": req.Context.MaxQuestions,
		},
	}
	return s.dariusClient.CallDarius(ctx, payload)
}

func (s *IreliaService) CallKarma(ctx context.Context, req *pb.LipSyncRequest) (*pb.LipSyncResponse, error) {
	payload := map[string]interface{}{
		"content": req.Content,
		"voiceId": req.VoiceId,
		"speed":   req.Speed,
	}
	return s.karmaClient.CallKarma(ctx, payload)
}
