package briefing

// The forge reads a run's producers make, as the fixtures under fixtures/ record them. In fullsend these come from
// forge.Client with a read-level mint token (ADR 0073) after the harness pre-script; here they are fictional.

// Read is what every agent's producers share: the run, the pinned agent definition and output contract, and the
// repository's AGENTS.md at the protected base branch's SHA.
type Read struct {
	Repo           string   `json:"repo"`
	BaseSHA        string   `json:"base_sha"`
	RunID          string   `json:"run_id"`
	ReadAt         string   `json:"read_at"`
	AssemblyTime   string   `json:"assembly_time"`
	Agent          Agent    `json:"agent"`
	OutputContract Contract `json:"output_contract"`
	Example        *Example `json:"example"`
	RepoAgentsMD   RepoFile `json:"repo_agents_md"`
}

// Agent is the agent definition at the SHA .fullsend/config.yaml pins (ADR 0058).
type Agent struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	SHA    string `json:"sha"`
	Body   string `json:"body"`
}

// Contract is the output schema the harness validation loop enforces (ADR 0022), as the model is told it.
type Contract struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Body    string `json:"body"`
}

// Example is a developer-owned steering example from the agents repository.
type Example struct {
	ID   string `json:"id"`
	Body string `json:"body"`
}

// RepoFile is a file read from the protected base branch, never from a pull request's head.
type RepoFile struct {
	Path string `json:"path"`
	Body string `json:"body"`
}

// Trigger is the prompting event and its actor's permission, as dispatch authorized it (ADR 0054, ADR 0098).
type Trigger struct {
	Kind            string `json:"kind"`
	Label           string `json:"label"`
	Actor           string `json:"actor"`
	ActorPermission string `json:"actor_permission"`
	At              string `json:"at"`
}

// IssueRead is a triage run's forge read.
type IssueRead struct {
	Read
	Issue               Issue        `json:"issue"`
	Trigger             Trigger      `json:"trigger"`
	InstructingRoles    []string     `json:"triage_permission_threshold"`
	Comments            []Comment    `json:"comments"`
	DuplicateCandidates []Candidate  `json:"duplicate_candidates"`
	Memory              []RunSummary `json:"memory"`
	StaleReads          []CachedRead `json:"stale_reads"`
}

// Issue is the work item.
type Issue struct {
	Number           int      `json:"number"`
	Title            string   `json:"title"`
	State            string   `json:"state"`
	Labels           []string `json:"labels"`
	Author           string   `json:"author"`
	AuthorPermission string   `json:"author_permission"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
	Body             string   `json:"body"`
}

// Comment is one thread comment with its author's current repository permission.
type Comment struct {
	ID         int    `json:"id"`
	Author     string `json:"author"`
	Permission string `json:"permission"`
	Bot        bool   `json:"bot"`
	CreatedAt  string `json:"created_at"`
	Body       string `json:"body"`
}

// Candidate is a possible duplicate the similarity index scored (ADR 0002, building block 5).
type Candidate struct {
	Number    int     `json:"number"`
	Title     string  `json:"title"`
	State     string  `json:"state"`
	UpdatedAt string  `json:"updated_at"`
	Score     float64 `json:"score"`
	Excerpt   string  `json:"excerpt"`
}

// RunSummary is an earlier run's conclusion on the same work item.
type RunSummary struct {
	Run     string `json:"run"`
	At      string `json:"at"`
	Expires string `json:"expires"`
	Summary string `json:"summary"`
}

// CachedRead is state read earlier and cached, which may be too old to use.
type CachedRead struct {
	ID     string `json:"id"`
	ReadAt string `json:"read_at"`
	Body   string `json:"body"`
}

// PullRead is a review run's forge read.
type PullRead struct {
	Read
	PR            PullRequest    `json:"pr"`
	Trigger       Trigger        `json:"trigger"`
	RiskTier1     Risk           `json:"risk_tier1"`
	Diff          []FileDiff     `json:"diff"`
	Checks        []Check        `json:"checks"`
	PriorFindings []PriorFinding `json:"prior_findings"`
}

// PullRequest is the change proposal. Description is author-controlled and read here only so a test can show that
// the security route never produces it.
type PullRequest struct {
	Number           int      `json:"number"`
	Title            string   `json:"title"`
	Author           string   `json:"author"`
	AuthorPermission string   `json:"author_permission"`
	Fork             bool     `json:"fork"`
	HeadSHA          string   `json:"head_sha"`
	BaseRef          string   `json:"base_ref"`
	Labels           []string `json:"labels"`
	LinkedIssue      int      `json:"linked_issue"`
	Description      string   `json:"description"`
}

// Risk is the deterministic tier-1 score the harness pre-script computes (ADR 0089).
type Risk struct {
	Score   int      `json:"score"`
	Signals []string `json:"signals"`
}

// FileDiff is one changed file at the head, with a diffstat line computed alongside it.
type FileDiff struct {
	Path string `json:"path"`
	Hunk string `json:"hunk"`
	Stat string `json:"stat"`
}

// Check is one CI check run's conclusion for one head SHA.
type Check struct {
	Name        string `json:"name"`
	HeadSHA     string `json:"head_sha"`
	Conclusion  string `json:"conclusion"`
	CompletedAt string `json:"completed_at"`
	Summary     string `json:"summary"`
}

// PriorFinding is a finding an earlier review of this pull request wrote. RevokedBy is set when a human dismissed
// it; Fact names the fact key when the finding is a factual claim another source can contradict.
type PriorFinding struct {
	ID        string `json:"id"`
	Run       string `json:"run"`
	At        string `json:"at"`
	Expires   string `json:"expires"`
	RevokedBy string `json:"revoked_by"`
	Fact      string `json:"fact"`
	Body      string `json:"body"`
}

// FixRead is a human-triggered fix run's forge read: the pull request, the /fs-fix instruction, the iteration
// count the pre-script checked, the diff and the findings the fix addresses.
type FixRead struct {
	Read
	PR               PullRequest      `json:"pr"`
	Trigger          Trigger          `json:"trigger"`
	HumanInstruction HumanInstruction `json:"human_instruction"`
	Iteration        Iteration        `json:"iteration"`
	Diff             []FileDiff       `json:"diff"`
	PriorFindings    []PriorFinding   `json:"prior_findings"`
}

// HumanInstruction is the free text after /fs-fix, which fullsend passes as HUMAN_INSTRUCTION, with the comment it
// came from and its author's current permission.
type HumanInstruction struct {
	CommentID  int    `json:"comment_id"`
	Author     string `json:"author"`
	Permission string `json:"permission"`
	At         string `json:"at"`
	Body       string `json:"body"`
}

// Iteration is the fix loop's count against its caps, which the harness pre-script enforces.
type Iteration struct {
	Human    int `json:"human"`
	Total    int `json:"total"`
	HumanCap int `json:"human_cap"`
	BotCap   int `json:"bot_cap"`
}
