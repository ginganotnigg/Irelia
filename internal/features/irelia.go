package features

import (
	"context"
	"fmt"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"sync"
	"time"

	pb "irelia/api"
	repo "irelia/internal/repo"
	sv "irelia/internal/service"
	ext "irelia/internal/utils/extractor"
	gen "irelia/internal/utils/generator"
	"irelia/internal/utils/redis"
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
	dariusClient       sv.DariusClient
	karmaClient        sv.KarmaClient
	repo               repo.Repository
	rabbit             rabbit.Rabbit
	logger             *zap.Logger
	extractor          ext.Extractor
	redis              redis.Redis
	questionWorkerPool *QuestionWorkerPool
	preparationMutex   sync.RWMutex
	preparationStatus  map[string]map[int32]bool
	timerManager       *QuestionTimerManager
}

// NewIrelia creates a new gRPC service for Frontend to Irelia communication
func New(repo *repo.Repository, rabbit rabbit.Rabbit, logger *zap.Logger, redis redis.Redis) *Irelia {
	ext := ext.New()
	dariusClient := sv.NewDariusClient(logger)
	karmaClient := sv.NewKarmaClient(logger)
	questionTimeout := viper.GetInt("question_timeout")
	timer := NewQuestionTimerManager(logger, time.Duration(questionTimeout)*time.Second)

	irelia := &Irelia{
		dariusClient: *dariusClient,
		karmaClient:  *karmaClient,
		repo:         *repo,
		rabbit:       rabbit,
		logger:       logger,
		extractor:    ext,
		redis:        redis,
		timerManager: timer,
	}
	size := viper.GetInt("worker.size")
	maxTasksPerWorker := viper.GetInt("worker.max_tasks_per_worker")
	maxIdleTime := viper.GetInt("worker.max_idle_time")
	maxTaskWaitTime := viper.GetInt("worker.max_task_wait_time")
	irelia.questionWorkerPool = NewQuestionWorkerPool(size, maxTasksPerWorker, maxIdleTime, maxTaskWaitTime)
	irelia.preparationStatus = make(map[string]map[int32]bool)
	irelia.questionWorkerPool.Start(irelia)
	return irelia
}

// StartInterview initializes a new interview session
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

	if err := s.repo.Interview.Create(ctx, userID, interview); err != nil {
		s.logger.Error("Failed to create interview", zap.Error(err))
		return nil, err
	}

	s.logger.Info("Created interview", zap.String("interviewId", interviewID))

	strings := []string{}
	introQuestion := s.generateIntroQuestion(interview.Language)
	if !req.SkipIntro {
		strings = append(strings, introQuestion)
	}

	fieldQuestion := s.generatePositionSpecificQuestions(interview.Position, interview.Language)
	strings = append(strings, fieldQuestion)

	questions := []*ent.Question{}
	for i, content := range strings {
		question := &ent.Question{
			QuestionIndex: int32(i + 1),
			InterviewID:   interviewID,
			Content:       content,
		}
		questions = append(questions, question)
	}

	// Retrieve the first question
	var firstIndex int32 = 1
	job := QuestionPreparationJob{
		InterviewID:    interviewID,
		UserID:         userID,
		NextQuestionID: firstIndex,
		Interview:      interview,
		Questions:      questions,
	}

	if err := s.prepareQuestion(ctx, job); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to prepare initial questions: %v", err)
	}

	// Prepare additional questions based on configuration
	nextJob := QuestionPreparationJob{
		InterviewID:    interviewID,
		UserID:         userID,
		NextQuestionID: int32(len(questions) + 1),
		Interview:      interview,
		Questions:      nil,
	}

	s.ensureQuestionWorkerPool()
	enqueued := s.questionWorkerPool.EnqueueJob(s.logger, nextJob)
	if !enqueued {
		s.logger.Warn("Failed to enqueue question preparation job",
			zap.String("interviewID", nextJob.InterviewID),
			zap.Int32("nextQuestionID", nextJob.NextQuestionID))
	}

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

	timerKey := fmt.Sprintf("%s:%d", req.InterviewId, req.Index)
	s.timerManager.cancelTimer(timerKey)

	question, err := s.repo.Question.Get(ctx, req.InterviewId, req.Index)
	if err != nil {
		s.logger.Error("Failed to retrieve question", zap.String("interviewId", req.InterviewId), zap.Int32("questionIndex", req.Index), zap.Error(err))
		return nil, status.Errorf(codes.NotFound, "Question not found: %v", err)
	}

	if question.Status != pb.QuestionStatus_QUESTION_STATUS_NEW {
		s.logger.Warn("Question already answered", zap.String("interviewId", req.InterviewId), zap.Int32("questionIndex", req.Index))
		return &pb.SubmitAnswerResponse{Message: "Question already answered"}, nil
	}
	if req.Answer == "" {
		s.logger.Warn("Answer is empty", zap.String("interviewId", req.InterviewId), zap.Int32("questionIndex", req.Index))
		return &pb.SubmitAnswerResponse{Message: "Answer is empty"}, nil
	}

	question.Answer = req.Answer
	question.RecordProof = req.RecordProof
	question.Status = pb.QuestionStatus_QUESTION_STATUS_ANSWERED

	if err := s.repo.Question.Update(ctx, userID, question); err != nil {
		s.logger.Warn("Failed to save answer", zap.Error(err))
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

	// Retrieve the next question from the database
	question, err := s.repo.Question.Get(ctx, req.InterviewId, req.QuestionIndex)
	if err != nil {
		s.logger.Warn("Failed to retrieve next question", zap.String("interviewId", req.InterviewId), zap.Int32("index", req.QuestionIndex), zap.Error(err))
		return &pb.QuestionResponse{
			QuestionId:     req.QuestionIndex,
			Content:        "",
			Audio:          "",
			Lipsync:        nil,
			IsLastQuestion: false,
			IsLoading:      true,
			Timestamp:      time.Now().Unix(),
		}, nil
	}

	// Determine if this is the last question
	isLastQuestion := req.QuestionIndex == interview.TotalQuestions

	s.timerManager.startTimer(req.InterviewId, req.QuestionIndex, userID, s.handleQuestionTimeout)

	if !isLastQuestion {
		// Prepare additional questions based on configuration
		job := QuestionPreparationJob{
			InterviewID:    req.InterviewId,
			UserID:         userID,
			NextQuestionID: req.QuestionIndex + 1,
			Interview:      interview,
			Questions:      nil,
		}

		s.ensureQuestionWorkerPool()
		enqueued := s.questionWorkerPool.EnqueueJob(s.logger, job)
		if !enqueued {
			s.logger.Warn("Failed to enqueue question preparation job",
				zap.String("interviewID", job.InterviewID),
				zap.Int32("nextQuestionID", job.NextQuestionID))
		}
	}

	// Return the next question
	return &pb.QuestionResponse{
		QuestionId:     question.QuestionIndex,
		Content:        question.Content,
		Audio:          question.Audio,
		Lipsync:        question.Lipsync,
		IsLastQuestion: isLastQuestion,
		IsLoading:      false,
		Timestamp:      time.Now().Unix(),
	}, nil
}

// SubmitInterview handles the submission of the entire interview
func (s *Irelia) SubmitInterview(ctx context.Context, req *pb.SubmitInterviewRequest) (*pb.SubmitInterviewResponse, error) {
	userID, err := s.getUserID(ctx)
	if err != nil {
		s.logger.Error("Failed to extract user ID from context", zap.Error(err))
		return nil, status.Errorf(codes.Unauthenticated, "Failed to extract user ID from context: %v", err)
	}

	s.timerManager.cleanupInterviewTimers(req.InterviewId)

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
	if err := s.repo.Interview.Update(ctx, userID, interview); err != nil {
		s.logger.Error("Failed to save interview status", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to save interview status: %v", err)
	}

	outro := &ent.Question{
		Content: s.prepareOutro(interview.Language),
	}

	// Prepare lip-sync data for the outro
	if outro, err = s.prepareLipSync(ctx, outro, interview, true, false); err != nil {
		s.logger.Error("Failed to prepare lip sync for the outro", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to prepare lip sync for the outro: %v", err)
	}

	// Get submissions from answers
	submissionsForDarius := make([]*pb.AnswerData, len(answers))
	submissionsForKarma := make([]*pb.AnswerData, len(answers))
	for i, answer := range answers {
		submissionsForDarius[i] = &pb.AnswerData{
			Index:    answer.Index,
			Question: &answer.Content,
			Answer:   answer.Answer,
		}
		submissionsForKarma[i] = &pb.AnswerData{
			Index:       answer.Index,
			Answer:      answer.Answer,
			RecordProof: &answer.RecordProof,
		}
	}

	// Get both request
	dariusReq := &pb.ScoreInterviewRequest{
		InterviewId: interview.ID,
		Submissions: submissionsForDarius,
		Skills:      interview.Skills,
	}
	karmaReq := &pb.ScoreFluencyRequest{
		InterviewId: interview.ID,
		Submissions: submissionsForKarma,
	}

	go func() {
		bgCtx := context.Background()
		dariusResp, err := s.callDariusForScore(bgCtx, userID, dariusReq)
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
			if ent.IsNotSingular(err) {
				s.logger.Error("Multiple questions found for the same index, skipping update",
					zap.String("interviewId", req.InterviewId),
					zap.Int32("questionIndex", submission.Index),
					zap.Error(err))
				continue
			}
			if err != nil {
				s.logger.Error("Failed to retrieve question", zap.String("interviewId", req.InterviewId), zap.Int32("questionIndex", submission.Index), zap.Error(err))
				continue
			}
			question.Comment = submission.Comment
			question.Score = submission.Score
			if question.Score == "" {
				question.Status = pb.QuestionStatus_QUESTION_STATUS_FAILED
			} else {
				question.Status = pb.QuestionStatus_QUESTION_STATUS_RATED
			}
			// Store question update list
			err = s.repo.Question.Update(bgCtx, userID, question)
			if err != nil {
				s.logger.Error("Failed to update question with score", zap.String("interviewId", req.InterviewId), zap.Int32("questionIndex", submission.Index), zap.Error(err))
				continue
			}
		}

		// Update the interview with feedback and total score
		totalLength := len(dariusResp.Skills) + len(karmaResp.Skills)
		skills := make([]string, 0, totalLength)
		skillsScore := make([]string, 0, totalLength)

		for _, ele := range dariusResp.Skills {
			skills = append(skills, ele.Skill)
			skillsScore = append(skillsScore, ele.Score)
		}

		for skill, score := range karmaResp.Skills {
			skills = append(skills, skill)
			skillsScore = append(skillsScore, score)
		}

		interview.Skills = skills
		interview.SkillsScore = skillsScore
		interview.TotalScore = dariusResp.TotalScore
		interview.PositiveFeedback = dariusResp.PositiveFeedback
		interview.ActionableFeedback = dariusResp.ActionableFeedback + " " + karmaResp.ActionableFeedback
		interview.FinalComment = dariusResp.FinalComment
		interview.Status = pb.InterviewStatus_INTERVIEW_STATUS_COMPLETED
		interview.OverallScore = getOverallScore(interview.TotalScore)

		if err := s.repo.Interview.Update(bgCtx, userID, interview); err != nil {
			s.logger.Error("Failed to save interview feedback", zap.Error(err))
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

	interviews, _, size, totalPages, err := s.repo.Interview.List(ctx, req, convertedUserId)
	if err != nil {
		s.logger.Error("Failed to retrieve interview history", zap.Error(err))
		return nil, fmt.Errorf("failed to retrieve interview history: %v", err)
	}

	var history []*pb.InterviewSummary
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

	return &pb.GetInterviewHistoryResponse{
		Page:       req.Page,
		PerPage:    size,
		TotalPages: totalPages,
		Interviews: history,
	}, nil
}

// FavoriteInterview marks an interview as favorite
func (s *Irelia) FavoriteInterview(ctx context.Context, req *pb.FavoriteInterviewRequest) (*emptypb.Empty, error) {
	userID, err := s.getUserID(ctx)
	if err != nil {
		s.logger.Error("Failed to extract user ID from context", zap.Error(err))
		return &emptypb.Empty{}, status.Errorf(codes.Unauthenticated, "Failed to extract user ID from context: %v", err)
	}

	return &emptypb.Empty{}, s.repo.Interview.Favorite(ctx, uint64(userID), req.InterviewId)
}

// DemoInterview is a demo method to start an interview with predefined parameters
func (s *Irelia) DemoInterview(ctx context.Context, req *pb.DemoRequest) (*pb.DemoResponse, error) {
	topic := req.Topic
	if topic == "" {
		topic = "basic-dsa"
	}
	questions, err := s.loadDemoQuestions(topic)
	if err != nil {
		s.logger.Error("Failed to load demo questions", zap.String("topic", topic), zap.Error(err))
		return nil, status.Errorf(codes.NotFound, "Demo topic not found: %v", err)
	}

	// Prepare QuestionResponse list
	var pbQuestions []*pb.QuestionResponse
	for i, q := range questions {
		pbQuestions = append(pbQuestions, &pb.QuestionResponse{
			QuestionId:     int32(i + 1),
			Content:        q.Content,
			Audio:          q.Audio,
			Lipsync:        q.Lipsync,
			IsLastQuestion: i == len(questions)-1,
			IsLoading:      false,
			Timestamp:      time.Now().Unix(),
		})
	}

	return &pb.DemoResponse{
		Questions: pbQuestions,
	}, nil
}

func (s *Irelia) GetPublicQuestion(ctx context.Context, req *pb.GetPublicQuestionRequest) (*pb.GetPublicQuestionResponse, error) {
	questions, totalCount, size, totalPages, err := s.repo.PublicQuestion.List(ctx, req)
	if err != nil {
		s.logger.Error("Failed to get public questions", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to get public questions: %v", err)
	}
	var pbQuestions []*pb.PublicQuestion
	for _, q := range questions {
		pbQuestions = append(pbQuestions, &pb.PublicQuestion{
			Content:    q.Content,
			Answer:     &q.Answer,
			Position:   q.Position,
			Experience: q.Experience,
			BaseData: &pb.BaseData{
				CreatedAt: timestamppb.New(q.CreatedAt),
				UpdatedAt: timestamppb.New(q.UpdatedAt),
			},
		})
	}
	return &pb.GetPublicQuestionResponse{
		Page:       req.Page,
		PerPage:    size,
		TotalPages: totalPages,
		TotalCount: totalCount,
		Questions:  pbQuestions,
	}, nil
}
