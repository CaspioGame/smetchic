package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"smetchic/application"
)

func (r *LifecycleRepository) CreateChat(ctx context.Context, chat application.ChatDialog) (application.ChatDialog, error) {
	err := r.db.QueryRowContext(ctx, `INSERT INTO chat_dialogs (id, company_id, user_id, title, state)
		VALUES ($1,$2,$3,$4,$5::jsonb) RETURNING updated_at`,
		chat.ID, chat.CompanyID, chat.UserID, chat.Title, string(chat.State)).Scan(&chat.UpdatedAt)
	return chat, err
}

func (r *LifecycleRepository) ListChats(ctx context.Context, companyID, userID string) ([]application.ChatDialog, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, title, updated_at FROM chat_dialogs
		WHERE company_id=$1 AND user_id=$2 ORDER BY updated_at DESC, id`, companyID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var chats []application.ChatDialog
	for rows.Next() {
		var chat application.ChatDialog
		if err := rows.Scan(&chat.ID, &chat.Title, &chat.UpdatedAt); err != nil {
			return nil, err
		}
		chats = append(chats, chat)
	}
	return chats, rows.Err()
}

func (r *LifecycleRepository) GetChat(ctx context.Context, companyID, userID, id string) (application.ChatDialog, error) {
	chat := application.ChatDialog{ID: id, CompanyID: companyID, UserID: userID}
	err := r.db.QueryRowContext(ctx, `SELECT title, state, revision, updated_at FROM chat_dialogs
		WHERE company_id=$1 AND user_id=$2 AND id=$3`, companyID, userID, id).
		Scan(&chat.Title, &chat.State, &chat.Revision, &chat.UpdatedAt)
	if err != nil {
		return application.ChatDialog{}, notFound("get chat", err)
	}
	return chat, nil
}

func (r *LifecycleRepository) ClaimChat(ctx context.Context, companyID, userID, id string, revision int) error {
	result, err := r.db.ExecContext(ctx, `UPDATE chat_dialogs SET revision=revision+1, busy_until=now()+interval '2 minutes'
		WHERE company_id=$1 AND user_id=$2 AND id=$3 AND revision=$4
		  AND (busy_until IS NULL OR busy_until < now())`, companyID, userID, id, revision)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return application.Invalid("Диалог уже обновился или обрабатывает сообщение. Обновите страницу.", nil)
	}
	return err
}

func (r *LifecycleRepository) SaveChat(ctx context.Context, chat application.ChatDialog) error {
	result, err := r.db.ExecContext(ctx, `UPDATE chat_dialogs SET title=$4, state=$5::jsonb,
		busy_until=CASE WHEN $7 THEN now()+interval '2 minutes' ELSE NULL END, updated_at=now()
		WHERE company_id=$1 AND user_id=$2 AND id=$3 AND revision=$6`,
		chat.CompanyID, chat.UserID, chat.ID, chat.Title, string(chat.State), chat.Revision, chat.Processing)
	if err != nil {
		return err
	}
	return requireAffected(result, "save chat")
}

func (r *LifecycleRepository) CreateChatEstimate(ctx context.Context, estimate application.EstimateRecord, part application.PartRecord, revision application.PartRevisionRecord) (application.EstimateRecord, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return application.EstimateRecord{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO parts(id,company_id,project_id,name)
		SELECT $1,$2,id,$4 FROM projects WHERE company_id=$2 AND id=$3`,
		part.ID, part.CompanyID, part.ProjectID, part.Name); err != nil {
		return application.EstimateRecord{}, err
	}
	hash := sha256.Sum256(nil)
	if _, err := tx.ExecContext(ctx, `INSERT INTO part_revisions
		(id,company_id,part_id,revision_number,source_format,source_filename,source_content,source_hash,parser_version,parsed_part_json)
		VALUES ($1,$2,$3,1,'manual','Без файла','',$4,$5,'{}')`,
		revision.ID, revision.CompanyID, revision.PartID, hex.EncodeToString(hash[:]), revision.ParserVersion); err != nil {
		return application.EstimateRecord{}, err
	}
	number, err := allocateEstimateNumber(ctx, tx, estimate.CompanyID)
	if err != nil {
		return application.EstimateRecord{}, err
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO estimates
		(id,company_id,part_revision_id,name,number,project_id,part_id,quantity,status,working_input,order_id)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,'draft',$9::jsonb,NULLIF($10,'')) RETURNING created_at,updated_at`,
		estimate.ID, estimate.CompanyID, revision.ID, estimate.Name, number, part.ProjectID, part.ID, estimate.Quantity, string(estimate.WorkingInput), estimate.OrderID).
		Scan(&estimate.CreatedAt, &estimate.UpdatedAt)
	if err != nil {
		return application.EstimateRecord{}, fmt.Errorf("create manual estimate: %w", err)
	}
	estimate.Number, estimate.Status = number, application.EstimateStatusDraft
	if err := tx.Commit(); err != nil {
		return application.EstimateRecord{}, err
	}
	return estimate, nil
}
