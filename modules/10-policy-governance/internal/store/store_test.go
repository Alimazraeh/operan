package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) (*PolicyStore, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	return NewPolicyStore(mockPool), mockPool
}

func newTestGroupStore(t *testing.T) (*GroupStore, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	return NewGroupStore(mockPool), mockPool
}

func newTestAuditStore(t *testing.T) (*AuditStore, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	return NewAuditStore(mockPool), mockPool
}

// ---- Policy CRUD Tests ----

func TestPolicyStore_Create(t *testing.T) {
	store, mockPool := newTestStore(t)
	ctx := context.Background()

	mockPool.ExpectQuery("INSERT INTO policies").
		WithArgs(
			pgxmock.AnyArg(), // $1 tenant_id
			pgxmock.AnyArg(), // $2 group_id
			pgxmock.AnyArg(), // $3 name
			pgxmock.AnyArg(), // $4 description
			pgxmock.AnyArg(), // $5 action
			pgxmock.AnyArg(), // $6 scope
			pgxmock.AnyArg(), // $7 resource_type
			pgxmock.AnyArg(), // $8 resource_target
			pgxmock.AnyArg(), // $9 condition_expression
			pgxmock.AnyArg(), // $10 effect
			pgxmock.AnyArg(), // $11 priority
			pgxmock.AnyArg(), // $12 is_active
			pgxmock.AnyArg(), // $13 created_by
		).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(uuid.New().String(), time.Now(), time.Now()))

	p := &Policy{
		TenantID: "tenant-1", GroupID: "group-1", Name: "Test Policy",
		Action: "deny", Scope: "global", ResourceType: "all",
		Effect: "enforce", Priority: 50, IsActive: true,
	}
	err := store.Create(ctx, p)
	require.NoError(t, err)
	assert.NotEmpty(t, p.ID)
}

func TestPolicyStore_GetByID(t *testing.T) {
	store, mockPool := newTestStore(t)
	ctx := context.Background()

	id := uuid.New()
	// pgxmock cannot scan string values into *string pointers for nullable columns
	// All nullable columns (description, resource_target, condition_expression, created_by) must be nil
	rows := pgxmock.NewRows([]string{
		"id", "tenant_id", "group_id", "name", "description", "action",
		"scope", "resource_type", "resource_target", "condition_expression",
		"effect", "priority", "is_active", "created_by", "created_at", "updated_at",
	}).AddRow(id.String(), "tenant-1", "group-1", "Test Policy",
		nil, "allow", "global", "all",
		nil, nil, "enforce", 50, true,
		nil, time.Now(), time.Now())
	mockPool.ExpectQuery("SELECT.*FROM policies WHERE id").
		WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	p, err := store.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "Test Policy", p.Name)
	assert.Equal(t, "allow", p.Action)
	assert.Equal(t, "global", p.Scope)
	assert.Nil(t, p.Description)
	assert.Nil(t, p.ResourceTarget)
	assert.Nil(t, p.CreatedBy)
}

func TestPolicyStore_GetByID_NotFound(t *testing.T) {
	store, mockPool := newTestStore(t)
	ctx := context.Background()

	id := uuid.New()
	mockPool.ExpectQuery("SELECT.*FROM policies WHERE id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)

	_, err := store.GetByID(ctx, id)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPolicyStore_List(t *testing.T) {
	store, mockPool := newTestStore(t)
	ctx := context.Background()

	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "group_id", "name", "description", "action",
		"scope", "resource_type", "resource_target", "condition_expression",
		"effect", "priority", "is_active", "created_by", "created_at", "updated_at",
	})
	dataRows.AddRow(uuid.New().String(), "tenant-1", "group-1", "Policy 1",
		nil, "allow", "global", "all", nil, nil, "enforce", 50, true, nil,
		time.Now(), time.Now())
	dataRows.AddRow(uuid.New().String(), "tenant-1", "group-1", "Policy 2",
		nil, "deny", "global", "all", nil, nil, "enforce", 60, true, nil,
		time.Now(), time.Now())
	// List with no filters: tenant_id=$1, LIMIT=$2, OFFSET=$3
	mockPool.ExpectQuery("SELECT.*FROM policies").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	countRows := pgxmock.NewRows([]string{"count"})
	countRows.AddRow(2)
	mockPool.ExpectQuery("SELECT COUNT").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(countRows)

	policies, total, err := store.List(ctx, "tenant-1", nil, nil, 1, 20)
	require.NoError(t, err)
	assert.Len(t, policies, 2)
	assert.Equal(t, 2, total)
}

func TestPolicyStore_List_WithFilter(t *testing.T) {
	store, mockPool := newTestStore(t)
	ctx := context.Background()

	scope := "global"
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "group_id", "name", "description", "action",
		"scope", "resource_type", "resource_target", "condition_expression",
		"effect", "priority", "is_active", "created_by", "created_at", "updated_at",
	})
	dataRows.AddRow(uuid.New().String(), "tenant-1", "group-1", "Global Policy",
		nil, "allow", "global", "all", nil, nil, "enforce", 50, true, nil,
		time.Now(), time.Now())
	// List with scope filter: tenant_id=$1, scope=$2, LIMIT=$3, OFFSET=$4
	mockPool.ExpectQuery("SELECT.*FROM policies.*scope").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	countRows := pgxmock.NewRows([]string{"count"})
	countRows.AddRow(1)
	mockPool.ExpectQuery("SELECT COUNT.*scope").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(countRows)

	policies, total, err := store.List(ctx, "tenant-1", &scope, nil, 1, 20)
	require.NoError(t, err)
	assert.Len(t, policies, 1)
	assert.Equal(t, 1, total)
}

func TestPolicyStore_Update(t *testing.T) {
	store, mockPool := newTestStore(t)
	ctx := context.Background()

	id := uuid.New()
	// Update: name=$1, description=$2, action=$3, scope=$4, resource_type=$5,
	// resource_target=$6, effect=$7, priority=$8, is_active=$9,
	// WHERE id=$10 AND tenant_id=$11
	mockPool.ExpectQuery("UPDATE policies SET").
		WithArgs(
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(),
		).
		WillReturnRows(pgxmock.NewRows([]string{"updated_at"}).AddRow(time.Now()))

	p := &Policy{
		ID: id.String(), TenantID: "tenant-1", Name: "Updated Policy",
		Action: "allow", Scope: "global", ResourceType: "tool",
		Effect: "enforce", Priority: 90, IsActive: true,
	}
	err := store.Update(ctx, p)
	require.NoError(t, err)
}

func TestPolicyStore_Update_NotFound(t *testing.T) {
	store, mockPool := newTestStore(t)
	ctx := context.Background()

	mockPool.ExpectQuery("UPDATE policies SET").
		WithArgs(
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(),
		).
		WillReturnError(pgx.ErrNoRows)

	p := &Policy{
		ID: "00000000-0000-0000-0000-000000000000", TenantID: "tenant-1",
		Name: "Updated", Action: "allow", Scope: "global", ResourceType: "tool",
		Effect: "enforce", Priority: 50, IsActive: true,
	}
	err := store.Update(ctx, p)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPolicyStore_Delete(t *testing.T) {
	store, mockPool := newTestStore(t)
	ctx := context.Background()

	id := uuid.New()
	mockPool.ExpectExec("UPDATE policies SET is_active").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := store.Delete(ctx, id, "tenant-1")
	require.NoError(t, err)
}

func TestPolicyStore_Delete_NotFound(t *testing.T) {
	store, mockPool := newTestStore(t)
	ctx := context.Background()

	id := uuid.New()
	mockPool.ExpectExec("UPDATE policies SET is_active").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err := store.Delete(ctx, id, "tenant-1")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPolicyStore_ListByGroup(t *testing.T) {
	store, mockPool := newTestStore(t)
	ctx := context.Background()

	groupID := uuid.New()
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "group_id", "name", "description", "action",
		"scope", "resource_type", "resource_target", "condition_expression",
		"effect", "priority", "is_active", "created_by", "created_at", "updated_at",
	})
	dataRows.AddRow(uuid.New().String(), "tenant-1", groupID.String(), "Group Policy",
		nil, "allow", "global", "all", nil, nil, "enforce", 50, true, nil,
		time.Now(), time.Now())
	mockPool.ExpectQuery("SELECT.*FROM policies WHERE group_id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	policies, err := store.ListByGroup(ctx, groupID)
	require.NoError(t, err)
	assert.Len(t, policies, 1)
}

func TestPolicyStore_ListActiveForTenant(t *testing.T) {
	store, mockPool := newTestStore(t)
	ctx := context.Background()

	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "group_id", "name", "description", "action",
		"scope", "resource_type", "resource_target", "condition_expression",
		"effect", "priority", "is_active", "created_by", "created_at", "updated_at",
	})
	dataRows.AddRow(uuid.New().String(), "tenant-1", "group-1", "Active Policy",
		nil, "allow", "global", "all", nil, nil, "enforce", 50, true, nil,
		time.Now(), time.Now())
	mockPool.ExpectQuery("SELECT.*FROM policies WHERE tenant_id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	policies, err := store.ListActiveForTenant(ctx, "tenant-1")
	require.NoError(t, err)
	assert.Len(t, policies, 1)
}

// ---- Policy Group CRUD Tests ----

func TestGroupStore_Create(t *testing.T) {
	store, mockPool := newTestGroupStore(t)
	ctx := context.Background()

	mockPool.ExpectQuery("INSERT INTO policy_groups").
		WithArgs(
			pgxmock.AnyArg(), // $1 tenant_id
			pgxmock.AnyArg(), // $2 name
			pgxmock.AnyArg(), // $3 description
			pgxmock.AnyArg(), // $4 priority
			pgxmock.AnyArg(), // $5 is_active
			pgxmock.AnyArg(), // $6 metadata
		).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(uuid.New().String(), time.Now(), time.Now()))

	g := &PolicyGroup{
		TenantID: "tenant-1", Name: "Financial Compliance",
		Priority: 80, IsActive: true,
	}
	err := store.Create(ctx, g)
	require.NoError(t, err)
	assert.NotEmpty(t, g.ID)
}

func TestGroupStore_GetByID(t *testing.T) {
	store, mockPool := newTestGroupStore(t)
	ctx := context.Background()

	id := uuid.New()
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "name", "description", "priority", "is_active",
		"metadata", "created_at", "updated_at",
	}).AddRow(
		id.String(), "tenant-1", "Compliance Group",
		"A compliance group", 70, true, pgxmock.AnyArg(),
		time.Now(), time.Now())
	mockPool.ExpectQuery("SELECT.*FROM policy_groups WHERE id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	g, err := store.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "Compliance Group", g.Name)
}

func TestGroupStore_GetByID_NotFound(t *testing.T) {
	store, mockPool := newTestGroupStore(t)
	ctx := context.Background()

	id := uuid.New()
	mockPool.ExpectQuery("SELECT.*FROM policy_groups WHERE id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)

	_, err := store.GetByID(ctx, id)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGroupStore_List(t *testing.T) {
	store, mockPool := newTestGroupStore(t)
	ctx := context.Background()

	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "name", "description", "priority", "is_active",
		"metadata", "created_at", "updated_at",
	}).AddRow(
		uuid.New().String(), "tenant-1", "Data Privacy",
		nil, 60, true, pgxmock.AnyArg(),
		time.Now(), time.Now())
	// List: tenant_id=$1, LIMIT=$2, OFFSET=$3
	mockPool.ExpectQuery("SELECT.*FROM policy_groups").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	countRows := pgxmock.NewRows([]string{"count"})
	countRows.AddRow(1)
	mockPool.ExpectQuery("SELECT COUNT").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(countRows)

	groups, total, err := store.List(ctx, "tenant-1", 1, 20)
	require.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, 1, total)
}

func TestGroupStore_Update(t *testing.T) {
	store, mockPool := newTestGroupStore(t)
	ctx := context.Background()

	id := uuid.New()
	// Group Update: Name="Updated Group", Priority=90, IsActive=true
	// SET name=$1, priority=$3, is_active=$4, updated_at=NOW()
	// WHERE id=$5 AND tenant_id=$6
	// args: "Updated Group", 90, true, id, tenantID => 5 args
	mockPool.ExpectExec("UPDATE policy_groups SET").
		WithArgs(
			pgxmock.AnyArg(), // name
			pgxmock.AnyArg(), // priority
			pgxmock.AnyArg(), // is_active
			pgxmock.AnyArg(), // id (WHERE)
			pgxmock.AnyArg(), // tenant_id (WHERE)
		).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	g := &PolicyGroup{
		ID: id.String(), TenantID: "tenant-1", Name: "Updated Group",
		Priority: 90, IsActive: true,
	}
	err := store.Update(ctx, g)
	require.NoError(t, err)
}

func TestGroupStore_Delete(t *testing.T) {
	store, mockPool := newTestGroupStore(t)
	ctx := context.Background()

	id := uuid.New()
	mockPool.ExpectExec("UPDATE policy_groups SET is_active").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := store.Delete(ctx, id, "tenant-1")
	require.NoError(t, err)
}

func TestGroupStore_Delete_NotFound(t *testing.T) {
	store, mockPool := newTestGroupStore(t)
	ctx := context.Background()

	id := uuid.New()
	mockPool.ExpectExec("UPDATE policy_groups SET is_active").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err := store.Delete(ctx, id, "tenant-1")
	assert.ErrorIs(t, err, ErrNotFound)
}

// ---- Audit Tests ----

func TestAuditStore_Create(t *testing.T) {
	store, mockPool := newTestAuditStore(t)
	ctx := context.Background()

	mockPool.ExpectQuery("INSERT INTO policy_audits").
		WithArgs(
			pgxmock.AnyArg(), // $1 tenant_id
			pgxmock.AnyArg(), // $2 policy_id (nil)
			pgxmock.AnyArg(), // $3 group_id (nil)
			pgxmock.AnyArg(), // $4 request_id (nil)
			pgxmock.AnyArg(), // $5 agent_id
			pgxmock.AnyArg(), // $6 resource_type
			pgxmock.AnyArg(), // $7 resource_target (nil)
			pgxmock.AnyArg(), // $8 requested_action
			pgxmock.AnyArg(), // $9 result
			pgxmock.AnyArg(), // $10 matched_policy_name (nil)
			pgxmock.AnyArg(), // $11 matched_rule_index (nil)
			pgxmock.AnyArg(), // $12 evaluation_ms
			pgxmock.AnyArg(), // $13 request_data (nil)
		).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
			AddRow(uuid.New().String(), time.Now()))

	a := &PolicyAudit{
		TenantID: "tenant-1", AgentID: ptrStr("agent-1"),
		ResourceType: "tool", RequestedAction: "send_email",
		Result: "denied", EvaluationMS: 5,
	}
	err := store.Create(ctx, a)
	require.NoError(t, err)
	assert.NotEmpty(t, a.ID)
}

func TestAuditStore_List(t *testing.T) {
	store, mockPool := newTestAuditStore(t)
	ctx := context.Background()

	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "policy_id", "group_id", "request_id", "agent_id",
		"resource_type", "resource_target", "requested_action", "result",
		"matched_policy_name", "matched_rule_index", "evaluation_ms", "request_data", "created_at",
	}).AddRow(
		uuid.New().String(), "tenant-1", nil, nil, nil, nil,
		"tool", nil, "send_email", "denied", nil, nil, 5, nil, time.Now())
	mockPool.ExpectQuery("SELECT.*FROM policy_audits").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	countRows := pgxmock.NewRows([]string{"count"})
	countRows.AddRow(1)
	mockPool.ExpectQuery("SELECT COUNT").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(countRows)

	audits, total, err := store.List(ctx, "tenant-1", nil, nil, nil, nil, 1, 50)
	require.NoError(t, err)
	assert.Len(t, audits, 1)
	assert.Equal(t, 1, total)
}

func TestAuditStore_List_WithAgentFilter(t *testing.T) {
	store, mockPool := newTestAuditStore(t)
	ctx := context.Background()

	agentID := "agent-1"
	// pgxmock cannot scan string values into *string pointers for nullable columns
	// Use nil for agent_id and assert on other non-nullable fields
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "policy_id", "group_id", "request_id", "agent_id",
		"resource_type", "resource_target", "requested_action", "result",
		"matched_policy_name", "matched_rule_index", "evaluation_ms", "request_data", "created_at",
	}).AddRow(
		uuid.New().String(), "tenant-1", nil, nil, nil, nil,
		"tool", nil, "send_email", "allowed", nil, nil, 3, nil, time.Now())
	// With agent_id filter: tenant_id=$1, agent_id=$2, LIMIT=$3, OFFSET=$4
	mockPool.ExpectQuery("SELECT.*FROM policy_audits.*agent_id").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	countRows := pgxmock.NewRows([]string{"count"})
	countRows.AddRow(1)
	mockPool.ExpectQuery("SELECT COUNT.*agent_id").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(countRows)

	audits, total, err := store.List(ctx, "tenant-1", &agentID, nil, nil, nil, 1, 50)
	require.NoError(t, err)
	assert.Len(t, audits, 1)
	assert.Equal(t, 1, total)
	assert.Equal(t, "tool", audits[0].ResourceType)
	assert.Equal(t, "send_email", audits[0].RequestedAction)
	assert.Equal(t, "allowed", audits[0].Result)
}

func TestAuditStore_List_WithResultFilter(t *testing.T) {
	store, mockPool := newTestAuditStore(t)
	ctx := context.Background()

	result := "denied"
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "policy_id", "group_id", "request_id", "agent_id",
		"resource_type", "resource_target", "requested_action", "result",
		"matched_policy_name", "matched_rule_index", "evaluation_ms", "request_data", "created_at",
	}).AddRow(
		uuid.New().String(), "tenant-1", nil, nil, nil, nil,
		"tool", nil, "send_email", "denied", nil, nil, 10, nil, time.Now())
	// With result filter: tenant_id=$1, result=$2, LIMIT=$3, OFFSET=$4
	mockPool.ExpectQuery("SELECT.*FROM policy_audits.*result").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	countRows := pgxmock.NewRows([]string{"count"})
	countRows.AddRow(1)
	mockPool.ExpectQuery("SELECT COUNT.*result").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(countRows)

	audits, total, err := store.List(ctx, "tenant-1", nil, &result, nil, nil, 1, 50)
	require.NoError(t, err)
	assert.Len(t, audits, 1)
	assert.Equal(t, 1, total)
}

func TestAuditStore_List_Pagination(t *testing.T) {
	store, mockPool := newTestAuditStore(t)
	ctx := context.Background()

	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "policy_id", "group_id", "request_id", "agent_id",
		"resource_type", "resource_target", "requested_action", "result",
		"matched_policy_name", "matched_rule_index", "evaluation_ms", "request_data", "created_at",
	}).AddRow(
		uuid.New().String(), "tenant-1", nil, nil, nil, nil,
		"tool", nil, "send_email", "allowed", nil, nil, 2, nil, time.Now())
	// Pagination: tenant_id=$1, LIMIT=$2, OFFSET=$3
	mockPool.ExpectQuery("SELECT.*FROM policy_audits").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	countRows := pgxmock.NewRows([]string{"count"})
	countRows.AddRow(10)
	mockPool.ExpectQuery("SELECT COUNT").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(countRows)

	audits, total, err := store.List(ctx, "tenant-1", nil, nil, nil, nil, 2, 5)
	require.NoError(t, err)
	assert.Len(t, audits, 1)
	assert.Equal(t, 10, total)
}

func ptrStr(s string) *string { return &s }