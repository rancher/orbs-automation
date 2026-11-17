package ship

import "github.com/rancher/orbs-automation/internal/proposal"

// These names moved to internal/proposal, the conveyor that ship and reposync share.
// They are aliases while ship is moved onto it, and are removed when it is.
type (
	Marker         = proposal.Marker
	PublishAction  = proposal.PublishAction
	Publish        = proposal.Publish
	PushRole       = proposal.PushRole
	PushChoice     = proposal.PushChoice
	PushRemotes    = proposal.PushRemotes
	PushTarget     = proposal.PushTarget
	Work           = proposal.Work
	Place          = proposal.Place
	GitState       = proposal.GitState
	GitCommit      = proposal.GitCommit
	BranchCommit   = proposal.BranchCommit
	Git            = proposal.Git
	GitCLI         = proposal.GitCLI
	PrepareRequest = proposal.PrepareRequest
	ResultStatus   = proposal.ResultStatus
	TargetResult   = proposal.TargetResult
	PRRef          = proposal.PRRef
	Apply          = proposal.Apply
	CommitInfo     = proposal.CommitInfo
	ApplyResult    = proposal.ApplyResult
)

const (
	PublishCreate      = proposal.PublishCreate
	PublishAlreadyOpen = proposal.PublishAlreadyOpen
	PublishBlocked     = proposal.PublishBlocked
	RoleFork           = proposal.RoleFork
	RoleUpstream       = proposal.RoleUpstream
	InPlace            = proposal.InPlace
	Worktree           = proposal.Worktree
	Existing           = proposal.Existing
	maxBumpBranches    = proposal.MaxBumpBranches

	ResultCommitted    = proposal.ResultCommitted
	ResultWouldCommit  = proposal.ResultWouldCommit
	ResultUpToDate     = proposal.ResultUpToDate
	ResultFailed       = proposal.ResultFailed
	ResultOpened       = proposal.ResultOpened
	ResultAlreadyOpen  = proposal.ResultAlreadyOpen
	ResultBlocked      = proposal.ResultBlocked
	ResultRefreshed    = proposal.ResultRefreshed
	ResultWouldRefresh = proposal.ResultWouldRefresh
)

var (
	ErrBaseMoved        = proposal.ErrBaseMoved
	ErrBranchExists     = proposal.ErrBranchExists
	ErrBranchCheckedOut = proposal.ErrBranchCheckedOut
	ErrDeclined         = proposal.ErrDeclined
	ErrNoRemote         = proposal.ErrNoRemote
	ErrNoClone          = proposal.ErrNoClone
	ErrNothingChanged   = proposal.ErrNothingChanged

	ParseMarker       = proposal.ParseMarker
	RecordedKey       = proposal.RecordedKey
	BranchKey         = proposal.BranchKey
	BranchTail        = proposal.BranchTail
	Prepare           = proposal.Prepare
	decidePublish     = proposal.DecidePublish
	decidePublishBy   = proposal.DecidePublishBy
	alsoNote          = proposal.AlsoNote
	otherBaseNote     = proposal.OtherBaseNote
	markerWarnings    = proposal.MarkerWarnings
	sourceOfKey       = proposal.SourceOfKey
	bumpBranches      = proposal.BumpBranches
	mergeBody         = proposal.MergeBody
	sameBody          = proposal.SameBody
	resolvePushTarget = proposal.ResolvePushTarget
	short             = proposal.Short
	refs              = proposal.Refs
	finish            = proposal.Finish
	tip               = proposal.Tip
	geroBlocks        = proposal.GeroBlocks
)
