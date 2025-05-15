package features

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "irelia/api"
	repo "irelia/internal/repo"
	sv "irelia/internal/service"
	ext "irelia/internal/utils/extractor"
	gen "irelia/internal/utils/generator"
	"irelia/internal/utils/tx"
	"irelia/pkg/ent"
	rabbit "irelia/pkg/rabbit/pkg"
)

type IIrelia interface {
	StartInterview(ctx context.Context, req *pb.StartInterviewRequest) (*pb.StartInterviewResponse, error)
	SubmitAnswer(ctx context.Context, req *pb.SubmitAnswerRequest) (*pb.SubmitAnswerResponse, error)
	GetNextQuestion(ctx context.Context, req *pb.QuestionRequest) (*pb.QuestionResponse, error)
	SubmitInterview(ctx context.Context, req *pb.SubmitInterviewRequest) (*pb.SubmitInterviewResponse, error)
	GetInterview(ctx context.Context, req *pb.GetInterviewRequest) (*pb.GetInterviewResponse, error)
	GetInterviewHistory(ctx context.Context, req *pb.GetInterviewHistoryRequest) (*pb.GetInterviewHistoryResponse, error)
	FavoriteInterview(ctx context.Context, req *pb.FavoriteInterviewRequest) (*emptypb.Empty, error)
}

// Irelia implements the InterviewService gRPC interface for Frontend to Irelia communication
type Irelia struct {
	pb.UnimplementedIreliaServer
	dariusClient sv.DariusClient
	karmaClient  sv.KarmaClient
	repo         repo.Repository
	rabbit       rabbit.Rabbit
	logger       *zap.Logger
	extractor    ext.Extractor
}

// NewIrelia creates a new gRPC service for Frontend to Irelia communication
func New(repo *repo.Repository, rabbit rabbit.Rabbit, logger *zap.Logger) *Irelia {
	ext := ext.New()
	dariusClient := sv.NewDariusClient(logger)
	karmaClient := sv.NewKarmaClient(logger)

	return &Irelia{
		dariusClient: *dariusClient,
		karmaClient:  *karmaClient,
		repo:         *repo,
		rabbit:       rabbit,
		logger:       logger,
		extractor:    ext,
	}
}

func (s *Irelia) StartInterview(ctx context.Context, req *pb.StartInterviewRequest) (*pb.StartInterviewResponse, error) {
	userID, err := s.getUserID(ctx)
	if err != nil {
		s.logger.Error("Failed to extract user ID from context", zap.Error(err))
		return nil, status.Errorf(codes.Unauthenticated, "Failed to extract user ID from context: %v", err)
	}

	interview := &ent.Interview{
		Position:           req.Position,
		Experience:         req.Experience,
		Language:           req.Language,
		VoiceID:            req.Models,
		Speed:              req.Speed,
		Skills:             req.Skills,
		SkipCode:           req.SkipCode,
		TotalQuestions:     req.TotalQuestions,
		RemainingQuestions: req.TotalQuestions,
	}

	// Generate a unique interview ID
	var interviewID string
	for {
		interviewID = gen.GenerateUUID()
		exists, err := s.repo.Interview.Exists(ctx, interviewID)
		if err != nil {
			s.logger.Error("Failed to query interview", zap.Error(err))
			return nil, fmt.Errorf("failed to query interview: %v", err)
		}
		if !exists {
			s.logger.Info("Generated unique interview ID", zap.String("interviewId", interviewID))
			break
		}
	}
	interview.ID = interviewID

	if txErr := tx.WithTransaction(ctx, s.repo.Ent, func(ctx context.Context, tx tx.Tx) error {
		err := s.repo.Interview.Create(ctx, tx, userID, interview)
		return err
	}); txErr != nil {
		s.logger.Error("Failed to create interview", zap.Error(txErr))
		return nil, txErr
	}

	s.logger.Info("Created interview", zap.String("interviewId", interviewID))

	questions := []string{}
	introQuestion := s.generateIntroQuestion(interview.Language)
	if !req.SkipIntro {
		questions = append(questions, introQuestion)
	}

	fieldQuestion := s.generatePositionSpecificQuestions(interview.Position, interview.Language)
	questions = append(questions, fieldQuestion)

	for i, content := range questions {
		question := &ent.Question{
			QuestionIndex: int32(i + 1),
			InterviewID:   interviewID,
			Content:       content,
		}
		if txErr := tx.WithTransaction(ctx, s.repo.Ent, func(ctx context.Context, tx tx.Tx) error {
			err := s.repo.Question.Create(ctx, tx, userID, question)
			return err
		}); txErr != nil {
			s.logger.Error("Failed to save question", zap.Error(txErr))
			return nil, txErr
		}
	}

	// Retrieve the first question
	firstQuestion, err := s.repo.Question.Get(ctx, interviewID, 1)
	if err != nil {
		s.logger.Error("Failed to retrieve first question", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to retrieve first question: %v", err)
	}

	// Prepare lip-sync data for the first question synchronously
	if err := s.prepareLipSync(ctx, firstQuestion, interview, false); err != nil {
		s.logger.Error("Failed to prepare lip sync for the first question", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to prepare lip sync for the first question: %v", err)
	}

	// Prepare additional questions based on configuration
	s.prepareQuestion(interviewID, userID, 1, interview)

	return &pb.StartInterviewResponse{
		InterviewId: interview.ID,
	}, nil
}

// SubmitAnswer handles the submission of an answer for a question
func (s *Irelia) SubmitAnswer(ctx context.Context, req *pb.SubmitAnswerRequest) (*pb.SubmitAnswerResponse, error) {
	userID, err := s.getUserID(ctx)
	if err != nil {
		s.logger.Error("Failed to extract user ID from context", zap.Error(err))
		return nil, status.Errorf(codes.Unauthenticated, "Failed to extract user ID from context: %v", err)
	}

	question, err := s.repo.Question.Get(ctx, req.InterviewId, req.Index)
	if err != nil {
		s.logger.Error("Failed to retrieve question", zap.String("interviewId", req.InterviewId), zap.Int32("questionIndex", req.Index), zap.Error(err))
		return nil, status.Errorf(codes.NotFound, "Question not found: %v", err)
	}

	if question.Status != pb.QuestionStatus_QUESTION_STATUS_NEW {
		s.logger.Error("Question already answered", zap.String("interviewId", req.InterviewId), zap.Int32("questionIndex", req.Index))
		return &pb.SubmitAnswerResponse{Message: "Question already answered"}, nil
	}
	if req.Answer == "" {
		s.logger.Error("Answer is empty", zap.String("interviewId", req.InterviewId), zap.Int32("questionIndex", req.Index))
		return &pb.SubmitAnswerResponse{Message: "Answer is empty"}, nil
	}

	question.Answer = req.Answer
	question.RecordProof = req.RecordProof
	question.Status = pb.QuestionStatus_QUESTION_STATUS_ANSWERED

	if txErr := tx.WithTransaction(ctx, s.repo.Ent, func(ctx context.Context, tx tx.Tx) error {
		err := s.repo.Question.Update(ctx, tx, userID, question)
		return err
	}); txErr != nil {
		s.logger.Error("Failed to save answer", zap.Error(txErr))
		return &pb.SubmitAnswerResponse{Message: "Failed to save answer"}, nil
	}

	return &pb.SubmitAnswerResponse{Message: "Answer submitted successfully"}, nil
}

// GetNextQuestion retrieves the next question for an interview
func (s *Irelia) GetNextQuestion(ctx context.Context, req *pb.QuestionRequest) (*pb.QuestionResponse, error) {
	userID, err := s.getUserID(ctx)
	if err != nil {
		s.logger.Error("Failed to extract user ID from context", zap.Error(err))
		return nil, status.Errorf(codes.Unauthenticated, "Failed to extract user ID from context: %v", err)
	}

	s.logger.Info("Retrieving next question", zap.String("interviewId", req.InterviewId), zap.Int32("index", req.QuestionIndex))

	// Retrieve the interview
	interview, err := s.repo.Interview.Get(ctx, req.InterviewId)
	if err != nil {
		s.logger.Error("Interview not found", zap.String("interviewId", req.InterviewId), zap.Error(err))
		return nil, status.Errorf(codes.NotFound, "Interview not found: %v", err)
	}

	if req.QuestionIndex > interview.TotalQuestions {
		return nil, status.Errorf(codes.InvalidArgument, "Question index out of range: %d", req.QuestionIndex)
	}

	// Retrieve the next question using the provided index
	exists, err := s.repo.Question.Exists(ctx, req.InterviewId, req.QuestionIndex)
	if err != nil {
		s.logger.Error("Failed to check if question exists", zap.String("interviewId", req.InterviewId), zap.Int32("index", req.QuestionIndex), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to retrieve next question: %v", err)
	}
	if !exists {
		s.logger.Error("Next question not found", zap.String("interviewId", req.InterviewId), zap.Int32("index", req.QuestionIndex), zap.Error(err))
	}

	// Retrieve the next question from the database
	question, err := s.repo.Question.Get(ctx, req.InterviewId, req.QuestionIndex)
	if err != nil {
		s.logger.Error("Failed to retrieve next question", zap.String("interviewId", req.InterviewId), zap.Int32("index", req.QuestionIndex), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to retrieve next question: %v", err)
	}

	// Prepare additional questions based on configuration
	s.prepareQuestion(req.InterviewId, userID, req.QuestionIndex, interview)

	// Determine if this is the last question
	isLastQuestion := req.QuestionIndex == interview.TotalQuestions

	// Return the next question
	return &pb.QuestionResponse{
		QuestionId:     question.QuestionIndex,
		Content:        question.Content,
		Audio:          question.Audio,
		Lipsync:        question.Lipsync,
		IsLastQuestion: isLastQuestion,
	}, nil
}

// SubmitInterview handles the submission of the entire interview
func (s *Irelia) SubmitInterview(ctx context.Context, req *pb.SubmitInterviewRequest) (*pb.SubmitInterviewResponse, error) {
	userID, err := s.getUserID(ctx)
	if err != nil {
		s.logger.Error("Failed to extract user ID from context", zap.Error(err))
		return nil, status.Errorf(codes.Unauthenticated, "Failed to extract user ID from context: %v", err)
	}

	interview, err := s.repo.Interview.Get(ctx, req.InterviewId)
	if err != nil {
		s.logger.Error("Failed to retrieve interview", zap.String("interviewId", req.InterviewId), zap.Error(err))
		return nil, fmt.Errorf("failed to retrieve interview: %v", err)
	}
	if interview.Status == pb.InterviewStatus_INTERVIEW_STATUS_COMPLETED {
		s.logger.Error("Interview already submitted", zap.String("interviewId", req.InterviewId))
		return nil, status.Errorf(codes.FailedPrecondition, "Interview already submitted: %v", err)
	}

	// Get all questions' AnswerData in interview
	answers, err := s.repo.Question.GetAnswers(ctx, req.InterviewId)
	if err != nil {
		s.logger.Error("Failed to retrieve answers", zap.Error(err))
	}

	// Save the interview status
	interview.Status = pb.InterviewStatus_INTERVIEW_STATUS_PENDING
	if txErr := tx.WithTransaction(ctx, s.repo.Ent, func(ctx context.Context, tx tx.Tx) error {
		err := s.repo.Interview.Update(ctx, tx, userID, interview)
		return err
	}); txErr != nil {
		s.logger.Error("Failed to save interview status", zap.Error(txErr))
		return nil, status.Errorf(codes.Internal, "Failed to save interview status: %v", txErr)
	}

	// Get submissions from answers
	submissionsForDarius := make([]*pb.AnswerData, len(answers))
	submissionsForKarma := make([]*pb.AnswerData, len(answers))
	for i, answer := range answers {
		submissionsForDarius[i] = &pb.AnswerData{
			Index:       answer.Index,
			Question:    &answer.Content,
			Answer:      answer.Answer,
		}
		submissionsForKarma[i] = &pb.AnswerData{
			Index:       answer.Index,
			Answer:      answer.Answer,
			RecordProof: &answer.RecordProof,
		}
	}

	// Get ScoreInterviewRequest
	dariusReq := &pb.ScoreInterviewRequest{
		InterviewId: interview.ID,
		Submissions: submissionsForDarius,
	}
	karmaReq := &pb.ScoreFluencyRequest{
		InterviewId: interview.ID,
		Submissions: submissionsForKarma,
	}

	outro := &ent.Question{
		Content: s.prepareOutro(interview.Language),
	}

	// Prepare lip-sync data for the outro
	if err := s.prepareLipSync(ctx, outro, interview, true); err != nil {
		s.logger.Error("Failed to prepare lip sync for the outro", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to prepare lip sync for the outro: %v", err)
	}

	go func() {
		bgCtx := context.Background()
        dariusResp, err := s.callDariusForScore(bgCtx, dariusReq)
        if err != nil {
            s.logger.Error("Failed to score by Darius", zap.Error(err))
            return
        }
        karmaResp, err := s.callKarmaForScore(bgCtx, karmaReq)
        if err != nil {
            s.logger.Error("Failed to score by Karma", zap.Error(err))
            return
        }

		// Update the database with scoring results
		interview.Status = pb.InterviewStatus_INTERVIEW_STATUS_PENDING
		for _, submission := range dariusResp.Result {
			question, err := s.repo.Question.Get(bgCtx, req.InterviewId, submission.Index)
			if err != nil {
				s.logger.Error("Failed to retrieve question", zap.String("interviewId", req.InterviewId), zap.Int32("questionIndex", submission.Index), zap.Error(err))
				return
			}
			question.Comment = submission.Comment
			question.Score = submission.Score
			if question.Score == "" {
				question.Status = pb.QuestionStatus_QUESTION_STATUS_FAILED
			} else {
				question.Status = pb.QuestionStatus_QUESTION_STATUS_RATED
			}
			if txErr := tx.WithTransaction(bgCtx, s.repo.Ent, func(bgCtx context.Context, tx tx.Tx) error {
				err := s.repo.Question.Update(bgCtx, tx, userID, question)
				return err
			}); txErr != nil {
				s.logger.Error("Failed to save interview status", zap.Error(txErr))
				return
			}
		}

		// Update the interview with feedback and total score
        skillSet := make(map[string]string) // Use a map to avoid duplicate skills
        for skill, score := range dariusResp.Skills {
            skillSet[skill] = score
        }
        for skill, score := range karmaResp.Skills {
            skillSet[skill] = score
        }

        // Convert the map to slices
        interview.Skills = make([]string, 0, len(skillSet))
        interview.SkillsScore = make([]string, 0, len(skillSet))
        for skill, score := range skillSet {
            interview.Skills = append(interview.Skills, skill)
            interview.SkillsScore = append(interview.SkillsScore, score)
        }
		
		interview.TotalScore = dariusResp.TotalScore
		interview.PositiveFeedback = dariusResp.PositiveFeedback
		interview.ActionableFeedback = dariusResp.ActionableFeedback + " " + karmaResp.ActionableFeedback
		interview.FinalComment = dariusResp.FinalComment
		interview.Status = pb.InterviewStatus_INTERVIEW_STATUS_COMPLETED

		if txErr := tx.WithTransaction(bgCtx, s.repo.Ent, func(bgCtx context.Context, tx tx.Tx) error {
			err := s.repo.Interview.Update(bgCtx, tx, userID, interview)
			return err
		}); txErr != nil {
			s.logger.Error("Failed to save interview feedback", zap.Error(txErr))
			return
		}
		s.logger.Info("Interview feedback saved successfully", zap.String("interviewId", interview.ID))
	}()

	return &pb.SubmitInterviewResponse{
		Outro: &pb.LipSyncResponse{
			Audio:   outro.Audio,
			Lipsync: outro.Lipsync,
		},
	}, nil
}

// GetInterview retrieves the details of a specific interview
func (s *Irelia) GetInterview(ctx context.Context, req *pb.GetInterviewRequest) (*pb.GetInterviewResponse, error) {
	entInterview, err := s.repo.Interview.Get(ctx, req.InterviewId)
	if err != nil {
		s.logger.Error("Failed to retrieve interview", zap.String("interviewId", req.InterviewId), zap.Error(err))
		return nil, fmt.Errorf("failed to retrieve interview: %v", err)
	}

	// Convert Ent Interview to Protobuf Interview
	submissions, err := s.repo.Question.List(ctx, req.InterviewId)
	if err != nil {
		s.logger.Error("Failed to retrieve questions", zap.String("interviewId", req.InterviewId), zap.Error(err))
		return nil, fmt.Errorf("failed to retrieve questions: %v", err)
	}


	skillsMap := make(map[string]string, len(entInterview.Skills))
	if len(entInterview.SkillsScore) == len(entInterview.Skills) {
		for i, skill := range entInterview.Skills {
			skillsMap[skill] = entInterview.SkillsScore[i]
		}
	}

	return &pb.GetInterviewResponse{
		InterviewId:        entInterview.ID,
		Submissions:        submissions,
		SkillsScore:        skillsMap,
		TotalScore:         entInterview.TotalScore,
		PositiveFeedback:   entInterview.PositiveFeedback,
		ActionableFeedback: entInterview.ActionableFeedback,
		FinalComment:       entInterview.FinalComment,
	}, nil
}

// GetInterviewHistory retrieves the history of interviews
func (s *Irelia) GetInterviewHistory(ctx context.Context, req *pb.GetInterviewHistoryRequest) (*pb.GetInterviewHistoryResponse, error) {
	var convertedUserId *uint64 = nil
	userID, err := s.getUserID(ctx)
	if err != nil {
		s.logger.Error("Failed to extract user ID from context", zap.Error(err))
		return nil, status.Errorf(codes.Unauthenticated, "Failed to extract user ID from context: %v", err)
	}
	if userID != 0 {
		temp := uint64(userID)
		convertedUserId = &temp
	}

	interviews, totalCount, totalPages, err := s.repo.Interview.List(ctx, req, convertedUserId)
	if err != nil {
		s.logger.Error("Failed to retrieve interview history", zap.Error(err))
		return nil, fmt.Errorf("failed to retrieve interview history: %v", err)
	}

	history := make([]*pb.InterviewSummary, 0, totalCount)
    for _, entInterview := range interviews {
        // Ensure all fields are correctly mapped
        history = append(history, &pb.InterviewSummary{
            InterviewId: entInterview.ID,
            Position:    entInterview.Position,
            Experience:  entInterview.Experience,
            TotalScore:  entInterview.TotalScore,
            BaseData: &pb.BaseData{
                CreatedAt: timestamppb.New(entInterview.CreatedAt),
                UpdatedAt: timestamppb.New(entInterview.UpdatedAt),
            },
        })
    }

	s.logger.Info("Interview history retrieved", zap.Int32("totalCount", totalCount), zap.Int32("totalPages", totalPages))


	return &pb.GetInterviewHistoryResponse{
		Page:       req.PageIndex,
		PerPage:    req.PageSize,
		TotalPages: totalPages,
		Interviews: history,
	}, nil
}

func (s *Irelia) FavoriteInterview(ctx context.Context, req *pb.FavoriteInterviewRequest) (*emptypb.Empty, error) {
	userID, err := s.getUserID(ctx)
	if err != nil {
		s.logger.Error("Failed to extract user ID from context", zap.Error(err))
		return &emptypb.Empty{}, status.Errorf(codes.Unauthenticated, "Failed to extract user ID from context: %v", err)
	}

	return &emptypb.Empty{}, s.repo.Interview.Favorite(ctx, uint64(userID), req.InterviewId)
}
