package automation

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"automatiza-go/internal/config"
	"automatiza-go/internal/lib/text"
	"automatiza-go/internal/models"
	"automatiza-go/internal/workers"

	"go.uber.org/zap"
)

// processListMode processes LIST mode automations.
// It fetches unconsumed items from the linked ML link list, processes each item
// (generates affiliate links, builds messages), and sends them to configured groups.
//
// LIST mode behavior (following Node.js parity):
// - Always processes 1 item per run per group (fixed for product consistency)
// - Always includes image when available
// - Items are consumed in order (order_index) and marked as consumed after sending
// - Deduplication is handled at the list level (consumed items are not reprocessed)
func (s *automationService) processListMode(ctx context.Context, automation *models.Automation) (*models.RunAutomationResult, error) {
	s.logger.Info("Processing LIST mode automation",
		zap.String("automationID", automation.ID),
		zap.String("automationName", automation.Name))

	// Validate ml_link_list_id is configured
	if automation.MlLinkListID == nil || strings.TrimSpace(*automation.MlLinkListID) == "" {
		errMsg := "Selecione uma Lista de Links para usar no modo Lista."
		s.logDispatch(ctx, automation, "system", "https://error.log", "error", errMsg)
		return &models.RunAutomationResult{
			Success: false,
			Message: errMsg,
		}, nil
	}

	listID := strings.TrimSpace(*automation.MlLinkListID)

	// Validate target groups are configured
	targetGroups := automation.GetTargetGroups()
	if len(targetGroups) == 0 {
		errMsg := "Nenhum grupo de envio configurado. Configure pelo menos um grupo na automacao."
		s.logDispatch(ctx, automation, "system", "https://error.log", "error", errMsg)
		return &models.RunAutomationResult{
			Success: false,
			Message: errMsg,
		}, nil
	}

	// Filter to only valid WhatsApp group JIDs
	validGroups := make([]string, 0, len(targetGroups))
	for _, gid := range targetGroups {
		if strings.Contains(gid, "@g.us") {
			validGroups = append(validGroups, gid)
		}
	}

	if len(validGroups) == 0 {
		errMsg := "Nenhum grupo valido configurado. Os grupos devem terminar com @g.us."
		s.logDispatch(ctx, automation, "system", "https://error.log", "error", errMsg)
		return &models.RunAutomationResult{
			Success: false,
			Message: errMsg,
		}, nil
	}

	// Get connected WhatsApp instance
	instances, err := s.instanceRepo.GetUserInstances(ctx, automation.UserID)
	if err != nil {
		s.logger.Error("Failed to get user instances",
			zap.String("userID", automation.UserID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get WhatsApp instances: %w", err)
	}

	var connectedInstance *models.Instance
	for i := range instances {
		if instances[i].Status == "connected" {
			connectedInstance = &instances[i]
			break
		}
	}

	if connectedInstance == nil {
		errMsg := "WhatsApp nao esta conectado. Conecte seu WhatsApp para enviar ofertas."
		s.logDispatch(ctx, automation, "system", "https://error.log", "error", errMsg)
		return &models.RunAutomationResult{
			Success: false,
			Message: errMsg,
		}, nil
	}

	// Get items per run (defaults to 1 for LIST mode per Node.js parity)
	// Clamped to 1-20 range to match frontend validation
	itemsPerRun := 1
	if automation.ListItemsPerRun != nil && *automation.ListItemsPerRun > 0 {
		itemsPerRun = *automation.ListItemsPerRun
	}
	if itemsPerRun > 20 {
		itemsPerRun = 20
	}

	// Include image (defaults to true for LIST mode per Node.js parity)
	includeImage := true
	if automation.ListIncludeImage != nil {
		includeImage = *automation.ListIncludeImage
	}

	// Get next unconsumed items from the list
	items, err := s.mlLinkListItemRepo.GetNextUnconsumed(ctx, listID, itemsPerRun)
	if err != nil {
		s.logger.Error("Failed to get unconsumed items",
			zap.String("listID", listID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get unconsumed items: %w", err)
	}

	if len(items) == 0 {
		infoMsg := "Lista sem itens disponiveis (todos consumidos ou inativos)."
		s.logger.Info("No unconsumed items in list",
			zap.String("listID", listID))
		s.logDispatch(ctx, automation, "system", "about:blank", "skipped", infoMsg)
		return &models.RunAutomationResult{
			Success:     true,
			Message:     infoMsg,
			OffersFound: 0,
		}, nil
	}

	// Get dispatch text template if configured (only if active)
	var dispatchTemplate string
	if automation.DispatchTextID != nil {
		dt, _ := s.settingsRepo.GetActiveDispatchTextByID(ctx, *automation.DispatchTextID, automation.UserID)
		if dt != nil {
			dispatchTemplate = dt.Body
		}
	}

	result := &models.RunAutomationResult{
		Success:      true,
		OffersFound:  len(items),
		MessagesSent: 0,
	}

	// Pre-fetch CTA entities if automation references cta_ids
	var ctaEntities []models.CTA
	if s.ctaRepo != nil && len(automation.CTAIDs) > 0 {
		fetched, ctaErr := s.ctaRepo.GetCTAsByIDs(ctx, automation.CTAIDs, automation.UserID)
		if ctaErr != nil {
			s.logger.Warn("Failed to fetch CTA entities, falling back to inline CTAs",
				zap.String("automationID", automation.ID),
				zap.Error(ctaErr))
		} else {
			ctaEntities = fetched
		}
	}

	// Process each item
	for _, item := range items {
		originURL := strings.TrimSpace(item.OriginURL)
		if originURL == "" {
			s.logger.Warn("Empty origin URL in list item, skipping",
				zap.String("itemID", item.ID))
			continue
		}

		// Detect platform
		isML := text.IsMLLink(originURL)
		isShopee := text.IsShopeeLink(originURL)
		if !isML && !isShopee {
			s.logDispatch(ctx, automation, "system", originURL, "skipped", "URL invalida (plataforma nao suportada).")
			continue
		}

		// Fetch product preview and generate affiliate link based on platform
		var preview *models.MLProductPreview
		var shopeeImageURL string
		affiliateURL := originURL

		if isML && s.mlService != nil {
			// ML: get preview + affiliate link
			p, err := s.mlService.GetProductPreview(ctx, originURL, automation.UserID)
			if err != nil {
				s.logger.Warn("Failed to get ML product preview",
					zap.String("originURL", originURL),
					zap.Error(err))
			} else {
				preview = p
			}

			link, err := s.mlService.CreateAffiliateLink(ctx, originURL, automation.UserID)
			if err != nil {
				s.logger.Warn("Failed to create ML affiliate link, using origin URL",
					zap.String("originURL", originURL),
					zap.Error(err))
			} else if link != nil && link.AffiliateLink != "" {
				affiliateURL = link.AffiliateLink
			}
		} else if isShopee && s.shopeeService != nil {
			// Shopee: generate affiliate link (no preview scraping in MVP)
			link, err := s.shopeeService.GenerateAffiliateLink(ctx, originURL, automation.UserID)
			if err != nil {
				s.logger.Warn("Failed to create Shopee affiliate link, using origin URL",
					zap.String("originURL", originURL),
					zap.Error(err))
			} else if link != nil && link.AffiliateLink != "" {
				affiliateURL = link.AffiliateLink
				if link.ImageURL != "" {
					shopeeImageURL = link.ImageURL
				}
			}
		}

		// Build message for this item
		message := s.buildListModeMessage(automation, item, preview, affiliateURL, dispatchTemplate, isShopee, ctaEntities)

		// Get image URL if available and enabled
		var imageURL *string
		if includeImage {
			if preview != nil && preview.Image != "" {
				imageURL = &preview.Image
			} else if shopeeImageURL != "" {
				imageURL = &shopeeImageURL
			} else if item.ProductImageURL != nil && *item.ProductImageURL != "" {
				imageURL = item.ProductImageURL
			}
		}

		// Build custom link preview for ML items with preview data
		var linkPreview *workers.LinkPreviewData
		if isML && preview != nil && preview.Title != "" {
			desc := fmt.Sprintf("R$ %.2f", preview.Price)
			if preview.OriginalPrice != nil {
				desc = fmt.Sprintf("De R$ %.2f por R$ %.2f", *preview.OriginalPrice, preview.Price)
			}
			imgURL := ""
			if preview.Image != "" {
				imgURL = strings.Replace(preview.Image, ".webp", ".jpg", 1)
			}
			linkPreview = &workers.LinkPreviewData{
				URL:         affiliateURL,
				Title:       preview.Title,
				Description: desc,
				ImageURL:    imgURL,
			}
			// Use link preview instead of image attachment
			imageURL = nil
		}

		// Send to each target group
		groupIDsSent := make([]string, 0, len(validGroups))
		for _, groupID := range validGroups {
			// Apply URL safety check
			if s.urlSafetyService != nil {
				decision, err := s.urlSafetyService.Evaluate(ctx, message)
				if err != nil {
					s.logger.Warn("URL safety check failed",
						zap.String("groupID", groupID),
						zap.Error(err))
				} else if decision != nil && decision.Action == "block" {
					reason := fmt.Sprintf("Link bloqueado por seguranca: %s", decision.Reason)
					s.logDispatch(ctx, automation, groupID, originURL, "skipped", reason)
					continue
				}
			}

			// Check backpressure
			if s.backpressure != nil {
				can, _, _ := s.backpressure.CanEnqueue(ctx, automation.UserID)
				if !can {
					s.logDispatch(ctx, automation, groupID, originURL, "skipped", "backpressure")
					continue
				}
			}

			// Create dispatch log BEFORE enqueueing so the worker can update its status
			var dispatchLogID *int
			if s.dispatchLogRepo != nil {
				featureFlags := config.GetFeatureFlags()
				if featureFlags.ShouldCreateDispatchLog() {
					dl := &models.DispatchLog{
						UserID:         automation.UserID,
						AutomationID:   &automation.ID,
						AutomationName: &automation.Name,
						GroupID:        &groupID,
						ProductURL:     &originURL,
						Status:         models.DispatchLogStatusQueued,
						CreatedAt:      time.Now().UTC().Format(time.RFC3339),
					}
					created, dlErr := s.dispatchLogRepo.Create(ctx, dl)
					if dlErr == nil && created != nil {
						dispatchLogID = &created.ID
					}
				}
			}

			// Enqueue message for sending
			jobData := &workers.MessageJobData{
				InstanceName:  connectedInstance.InstanceID,
				GroupID:       groupID,
				Message:       message,
				ImageURL:      imageURL,
				UserID:        automation.UserID,
				AutomationID:  automation.ID,
				DispatchLogID: dispatchLogID,
				LinkPreview:   linkPreview,
			}

			if s.queueClient != nil {
				// Calculate delay for this group (spread messages)
				delayMs := time.Duration(len(groupIDsSent)*3) * time.Second

				_, err := s.queueClient.EnqueueMessage(ctx, jobData, delayMs)
				if err != nil {
					s.logger.Error("Failed to enqueue message",
						zap.String("groupID", groupID),
						zap.Error(err))
					s.logDispatch(ctx, automation, groupID, originURL, "failed", err.Error())
					continue
				}
			}

			// Record backpressure
			if s.backpressure != nil {
				_ = s.backpressure.RecordEnqueue(ctx, automation.UserID)
			}
			groupIDsSent = append(groupIDsSent, groupID)
			result.MessagesSent++
		}

		// Mark item as consumed (only if we sent to at least one group)
		if len(groupIDsSent) > 0 {
			// Use the first group ID for tracking (the item is consumed for all groups)
			firstGroupID := groupIDsSent[0]
			err := s.mlLinkListItemRepo.MarkAsConsumed(ctx, []string{item.ID}, automation.ID, &firstGroupID)
			if err != nil {
				s.logger.Error("Failed to mark item as consumed",
					zap.String("itemID", item.ID),
					zap.Error(err))
			}
		}
	}

	if result.MessagesSent > 0 {
		result.Message = fmt.Sprintf("Automacao executada: %d mensagem(s) enfileirada(s)", result.MessagesSent)
	} else {
		result.Message = "Nenhuma mensagem foi enviada"
	}

	return result, nil
}

// buildListModeMessage builds the message for a LIST mode item
func (s *automationService) buildListModeMessage(
	automation *models.Automation,
	item *models.MlLinkListItem,
	preview *models.MLProductPreview,
	affiliateURL string,
	dispatchTemplate string,
	isShopee bool,
	ctaEntities []models.CTA,
) string {
	// Select random CTA: prefer CTA entities, fallback to inline CTAs
	cta := ""
	if len(ctaEntities) > 0 {
		cta = ctaEntities[rand.IntN(len(ctaEntities))].Message
	} else if len(automation.CTAs) > 0 {
		cta = automation.CTAs[rand.IntN(len(automation.CTAs))]
	}

	if isShopee {
		// Shopee: use ShopeeOfferVars
		vars := text.ShopeeOfferVars{
			Link:          item.OriginURL,
			AffiliateLink: affiliateURL,
			CTA:           cta,
		}
		// Use item metadata if available
		if item.ProductTitle != nil {
			vars.Title = *item.ProductTitle
		}
		if item.ProductPrice != nil {
			vars.Price = *item.ProductPrice
		}
		if item.ProductImageURL != nil {
			vars.Image = *item.ProductImageURL
		}

		var message string
		if dispatchTemplate != "" {
			message = text.RenderShopeeOfferTemplate(dispatchTemplate, vars)
		} else {
			message = text.RenderShopeeOfferTemplate(text.DefaultShopeeTemplate(), vars)
		}

		// Auto-append CTA if template doesn't contain {cta} variable
		if cta != "" && !strings.Contains(dispatchTemplate, "{cta}") {
			message = message + "\n\n" + cta
		}

		return text.NormalizeOutgoingWhatsAppText(message)
	}

	// ML: use MLOfferVars (original logic)
	vars := text.MLOfferVars{
		Link:          item.OriginURL,
		AffiliateLink: affiliateURL,
		CTA:           cta,
	}

	// Use preview data if available, otherwise fall back to item metadata
	if preview != nil {
		vars.Title = preview.Title
		vars.Price = preview.Price
		if preview.OriginalPrice != nil {
			vars.OriginalPrice = *preview.OriginalPrice
		}
		vars.Discount = preview.Discount
		vars.Image = preview.Image
		if preview.Condition != nil {
			vars.Condition = *preview.Condition
		}
		if preview.SoldCount != nil {
			vars.SoldCount = *preview.SoldCount
		}
		if preview.Rating != nil {
			vars.Rating = *preview.Rating
		}
		if preview.ReviewCount != nil {
			vars.ReviewCount = *preview.ReviewCount
		}
		if preview.Installments != nil {
			vars.Installments = *preview.Installments
		}
	} else {
		if item.ProductTitle != nil {
			vars.Title = *item.ProductTitle
		}
		if item.ProductPrice != nil {
			vars.Price = *item.ProductPrice
		}
		if item.ProductImageURL != nil {
			vars.Image = *item.ProductImageURL
		}
	}

	var message string
	if dispatchTemplate != "" {
		message = text.RenderMLOfferTemplate(dispatchTemplate, vars)
	} else {
		message = text.RenderMLOfferTemplate(text.DefaultMLTemplate(), vars)
	}

	// Auto-append CTA if template doesn't contain {cta} variable
	if cta != "" && !strings.Contains(dispatchTemplate, "{cta}") {
		message = message + "\n\n" + cta
	}

	return text.NormalizeOutgoingWhatsAppText(message)
}
