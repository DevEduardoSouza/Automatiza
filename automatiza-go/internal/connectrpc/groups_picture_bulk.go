package connectrpc

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"sync"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	groupsv1 "automatiza-go/internal/gen/groups/v1"
	"automatiza-go/internal/models"

	"connectrpc.com/connect"
)

// bulkPicturePaceDelay is the delay between consecutive group-picture changes
// on the SAME chip. WhatsApp rejects rapid successive photo changes with
// "info query returned status 401: not-authorized", so we space them out.
// Chips run in parallel, so overall wall-clock is bounded by the busiest chip,
// not the total number of groups.
var bulkPicturePaceDelay = 2 * time.Second

// pickAdminChipForGroup returns the session name of a chip that administers the
// group and is currently connected. adminIDs is the group's admin_instance_ids
// (stringified); connectedByID maps chip id -> session name for connected chips
// only. A sticky FNV hash keeps the same group on the same chip across runs —
// what WhatsApp's anti-ban heuristics expect (consistent sender per group).
// Returns "" when no admin chip is connected, which the caller surfaces as a
// per-group error instead of letting WhatsApp reject with "not-authorized".
func pickAdminChipForGroup(waGroupID string, adminIDs []string, connectedByID map[string]string) string {
	eligible := make([]string, 0, len(adminIDs))
	for _, id := range adminIDs {
		if _, ok := connectedByID[id]; ok {
			eligible = append(eligible, id)
		}
	}
	if len(eligible) == 0 {
		return ""
	}
	sort.Strings(eligible) // deterministic order so the hash is stable

	h := fnv.New32a()
	_, _ = h.Write([]byte(waGroupID))
	idx := h.Sum32() % uint32(len(eligible))
	return connectedByID[eligible[idx]]
}

// UpdateGroupPictures updates the picture of many groups at once. Each group is
// routed through a chip that administers it (multi-chip aware), and changes are
// paced per chip to avoid WhatsApp anti-spam. Chips run in parallel. Returns a
// per-group result — one group failing never blocks the others.
func (h *GroupsHandler) UpdateGroupPictures(
	ctx context.Context,
	req *connect.Request[groupsv1.UpdateGroupPicturesRequest],
) (*connect.Response[groupsv1.UpdateGroupPicturesResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	groupIDs := req.Msg.GetGroupIds()
	if len(groupIDs) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("at least one group_id is required"))
	}
	imageData := req.Msg.GetBase64Data()
	if imageData == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("base64_data is required"))
	}
	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	// Map connected chip id -> session name.
	instances, err := h.instanceRepo.GetUserInstances(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instances: %w", err))
	}
	connectedByID := make(map[string]string, len(instances))
	for _, inst := range instances {
		if inst.Status == models.InstanceStatusConnected {
			connectedByID[strconv.Itoa(inst.ID)] = inst.InstanceID
		}
	}

	// Which chips administer each group.
	adminByGroup, err := h.groupRepo.GetAdminInstanceIDsForGroups(ctx, user.ID, groupIDs)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get group admins: %w", err))
	}

	// Route each group to an admin chip and bucket by chosen session. Groups
	// with no eligible chip get an immediate error result and are skipped.
	resultByGroup := make(map[string]*groupsv1.UpdateGroupPictureResult, len(groupIDs))
	byChip := make(map[string][]*groupsv1.UpdateGroupPictureResult)
	for _, gid := range groupIDs {
		session := pickAdminChipForGroup(gid, adminByGroup[gid], connectedByID)
		res := &groupsv1.UpdateGroupPictureResult{GroupId: gid, UsedInstance: session}
		resultByGroup[gid] = res
		if session == "" {
			res.Error = "nenhum chip admin conectado para este grupo"
			continue
		}
		byChip[session] = append(byChip[session], res)
	}

	// Fan out per chip in parallel; pace changes within each chip. Each result
	// pointer is owned by exactly one goroutine, so mutating res fields needs
	// no lock — only recordHistory/assembly below runs after wg.Wait().
	var wg sync.WaitGroup
	for session, items := range byChip {
		wg.Add(1)
		go func(session string, items []*groupsv1.UpdateGroupPictureResult) {
			defer wg.Done()
			for i, res := range items {
				if i > 0 {
					select {
					case <-ctx.Done():
						for _, r := range items[i:] {
							r.Error = "cancelado"
						}
						return
					case <-time.After(bulkPicturePaceDelay):
					}
				}
				perr := h.waProvider.UpdateGroupPicture(ctx, whatsapp.UpdateGroupPictureInput{
					SessionName: session,
					GroupID:     res.GroupId,
					ImageBase64: imageData,
				})
				if perr != nil {
					res.Error = perr.Error()
				} else {
					res.Success = true
				}
			}
		}(session, items)
	}
	wg.Wait()

	// Assemble response in input order and record history for successes.
	results := make([]*groupsv1.UpdateGroupPictureResult, 0, len(groupIDs))
	var successCount, failedCount int32
	for _, gid := range groupIDs {
		res := resultByGroup[gid]
		results = append(results, res)
		if res.Success {
			successCount++
			h.recordHistory(ctx, user.ID, models.ActionTypeChangePhoto,
				"Alterar foto do grupo",
				map[string]interface{}{"group_id": gid})
		} else {
			failedCount++
		}
	}

	return connect.NewResponse(&groupsv1.UpdateGroupPicturesResponse{
		Results:      results,
		SuccessCount: successCount,
		FailedCount:  failedCount,
	}), nil
}
