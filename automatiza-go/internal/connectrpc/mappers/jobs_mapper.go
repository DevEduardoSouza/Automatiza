// Package mappers provides conversion functions between proto messages and domain models.
package mappers

import (
	jobsv1 "automatiza-go/internal/gen/jobs/v1"
	"automatiza-go/internal/models"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// JobToProto converts a domain StandaloneJob to proto Job.
func JobToProto(job *models.StandaloneJob) *jobsv1.Job {
	if job == nil {
		return nil
	}

	pb := &jobsv1.Job{
		Id:           job.ID,
		ActionType:   ActionTypeToProto(job.ActionType),
		ScheduledFor: timestamppb.New(job.ScheduledFor),
		Status:       StatusToProto(job.Status),
		Payload:      PayloadToProto(&job.Payload),
		CreatedAt:    timestamppb.New(job.CreatedAt),
		UpdatedAt:    timestamppb.New(job.UpdatedAt),
	}

	if job.ErrorMessage != nil {
		pb.ErrorMessage = job.ErrorMessage
	}

	return pb
}

// ProtoToJob converts a proto Job to domain StandaloneJob.
func ProtoToJob(pb *jobsv1.Job) *models.StandaloneJob {
	if pb == nil {
		return nil
	}

	job := &models.StandaloneJob{
		ID:         pb.Id,
		ActionType: ProtoToActionType(pb.ActionType),
		Status:     ProtoToStatus(pb.Status),
	}

	if pb.ErrorMessage != nil {
		job.ErrorMessage = pb.ErrorMessage
	}
	if pb.ScheduledFor != nil {
		job.ScheduledFor = pb.ScheduledFor.AsTime()
	}
	if pb.CreatedAt != nil {
		job.CreatedAt = pb.CreatedAt.AsTime()
	}
	if pb.UpdatedAt != nil {
		job.UpdatedAt = pb.UpdatedAt.AsTime()
	}
	if pb.Payload != nil {
		job.Payload = *ProtoToPayload(pb.Payload)
	}

	return job
}

// JobsToProto converts a slice of domain jobs to proto jobs.
func JobsToProto(jobs []models.StandaloneJob) []*jobsv1.Job {
	result := make([]*jobsv1.Job, len(jobs))
	for i := range jobs {
		result[i] = JobToProto(&jobs[i])
	}
	return result
}

// ActionTypeToProto converts domain JobActionType to proto enum.
func ActionTypeToProto(at models.JobActionType) jobsv1.JobActionType {
	switch at {
	case models.JobActionSendMessage:
		return jobsv1.JobActionType_JOB_ACTION_TYPE_SEND_MESSAGE
	case models.JobActionSendPoll:
		return jobsv1.JobActionType_JOB_ACTION_TYPE_SEND_POLL
	case models.JobActionSendMLOffer:
		return jobsv1.JobActionType_JOB_ACTION_TYPE_SEND_ML_OFFER
	case models.JobActionUpdateDescription:
		return jobsv1.JobActionType_JOB_ACTION_TYPE_UPDATE_DESCRIPTION
	case models.JobActionCloseGroups:
		return jobsv1.JobActionType_JOB_ACTION_TYPE_CLOSE_GROUPS
	case models.JobActionOpenGroups:
		return jobsv1.JobActionType_JOB_ACTION_TYPE_OPEN_GROUPS
	case models.JobActionChangeGroupName:
		return jobsv1.JobActionType_JOB_ACTION_TYPE_CHANGE_GROUP_NAME
	case models.JobActionChangeGroupPhoto:
		return jobsv1.JobActionType_JOB_ACTION_TYPE_CHANGE_GROUP_PHOTO
	default:
		return jobsv1.JobActionType_JOB_ACTION_TYPE_UNSPECIFIED
	}
}

// ProtoToActionType converts proto enum to domain JobActionType.
func ProtoToActionType(at jobsv1.JobActionType) models.JobActionType {
	switch at {
	case jobsv1.JobActionType_JOB_ACTION_TYPE_SEND_MESSAGE:
		return models.JobActionSendMessage
	case jobsv1.JobActionType_JOB_ACTION_TYPE_SEND_POLL:
		return models.JobActionSendPoll
	case jobsv1.JobActionType_JOB_ACTION_TYPE_SEND_ML_OFFER:
		return models.JobActionSendMLOffer
	case jobsv1.JobActionType_JOB_ACTION_TYPE_UPDATE_DESCRIPTION:
		return models.JobActionUpdateDescription
	case jobsv1.JobActionType_JOB_ACTION_TYPE_CLOSE_GROUPS:
		return models.JobActionCloseGroups
	case jobsv1.JobActionType_JOB_ACTION_TYPE_OPEN_GROUPS:
		return models.JobActionOpenGroups
	case jobsv1.JobActionType_JOB_ACTION_TYPE_CHANGE_GROUP_NAME:
		return models.JobActionChangeGroupName
	case jobsv1.JobActionType_JOB_ACTION_TYPE_CHANGE_GROUP_PHOTO:
		return models.JobActionChangeGroupPhoto
	default:
		return ""
	}
}

// StatusToProto converts domain StandaloneJobStatus to proto enum.
func StatusToProto(s models.StandaloneJobStatus) jobsv1.JobStatus {
	switch s {
	case models.StandaloneJobStatusPending:
		return jobsv1.JobStatus_JOB_STATUS_PENDING
	case models.StandaloneJobStatusRunning:
		return jobsv1.JobStatus_JOB_STATUS_RUNNING
	case models.StandaloneJobStatusDone:
		return jobsv1.JobStatus_JOB_STATUS_DONE
	case models.StandaloneJobStatusFailed:
		return jobsv1.JobStatus_JOB_STATUS_FAILED
	case models.StandaloneJobStatusCancelled:
		return jobsv1.JobStatus_JOB_STATUS_CANCELLED
	default:
		return jobsv1.JobStatus_JOB_STATUS_UNSPECIFIED
	}
}

// ProtoToStatus converts proto enum to domain StandaloneJobStatus.
func ProtoToStatus(s jobsv1.JobStatus) models.StandaloneJobStatus {
	switch s {
	case jobsv1.JobStatus_JOB_STATUS_PENDING:
		return models.StandaloneJobStatusPending
	case jobsv1.JobStatus_JOB_STATUS_RUNNING:
		return models.StandaloneJobStatusRunning
	case jobsv1.JobStatus_JOB_STATUS_DONE:
		return models.StandaloneJobStatusDone
	case jobsv1.JobStatus_JOB_STATUS_FAILED:
		return models.StandaloneJobStatusFailed
	case jobsv1.JobStatus_JOB_STATUS_CANCELLED:
		return models.StandaloneJobStatusCancelled
	default:
		return models.StandaloneJobStatusPending
	}
}

// PayloadToProto converts domain JobPayload to proto.
func PayloadToProto(p *models.JobPayload) *jobsv1.JobPayload {
	if p == nil {
		return nil
	}

	pb := &jobsv1.JobPayload{
		Groups:  p.Groups,
		Options: p.Options,
	}

	// Optional fields - only set if non-empty
	if p.Message != "" {
		pb.Message = &p.Message
	}
	if p.Media != "" {
		pb.Media = &p.Media
	}
	if p.MediaType != "" {
		pb.MediaType = &p.MediaType
	}
	if p.FileName != "" {
		pb.FileName = &p.FileName
	}
	if p.MimeType != "" {
		pb.MimeType = &p.MimeType
	}
	if p.Type != "" {
		pb.Type = &p.Type
	}
	if p.Question != "" {
		pb.Question = &p.Question
	}
	if p.SelectableCount > 0 {
		sc := int32(p.SelectableCount)
		pb.SelectableCount = &sc
	}
	if p.OriginURL != "" {
		pb.OriginUrl = &p.OriginURL
	}
	if p.Text != "" {
		pb.Text = &p.Text
	}
	if p.IncludeImage != nil {
		pb.IncludeImage = p.IncludeImage
	}
	if p.Description != "" {
		pb.Description = &p.Description
	}
	if p.Name != "" {
		pb.Name = &p.Name
	}
	if p.AutoNumber {
		pb.AutoNumber = &p.AutoNumber
	}
	if p.Image != "" {
		pb.Image = &p.Image
	}

	return pb
}

// ProtoToPayload converts proto JobPayload to domain.
func ProtoToPayload(pb *jobsv1.JobPayload) *models.JobPayload {
	if pb == nil {
		return nil
	}

	p := &models.JobPayload{
		Groups:  pb.Groups,
		Options: pb.Options,
	}

	if pb.Message != nil {
		p.Message = *pb.Message
	}
	if pb.Media != nil {
		p.Media = *pb.Media
	}
	if pb.MediaType != nil {
		p.MediaType = *pb.MediaType
	}
	if pb.FileName != nil {
		p.FileName = *pb.FileName
	}
	if pb.MimeType != nil {
		p.MimeType = *pb.MimeType
	}
	if pb.Type != nil {
		p.Type = *pb.Type
	}
	if pb.Question != nil {
		p.Question = *pb.Question
	}
	if pb.SelectableCount != nil {
		p.SelectableCount = int(*pb.SelectableCount)
	}
	if pb.OriginUrl != nil {
		p.OriginURL = *pb.OriginUrl
	}
	if pb.Text != nil {
		p.Text = *pb.Text
	}
	if pb.IncludeImage != nil {
		p.IncludeImage = pb.IncludeImage
	}
	if pb.Description != nil {
		p.Description = *pb.Description
	}
	if pb.Name != nil {
		p.Name = *pb.Name
	}
	if pb.AutoNumber != nil {
		p.AutoNumber = *pb.AutoNumber
	}
	if pb.Image != nil {
		p.Image = *pb.Image
	}

	return p
}
