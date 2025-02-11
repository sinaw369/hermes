package message

// BackMsg is sent when the user presses 'Esc' to navigate back to the previous screen
type BackMsg struct{}

// GitRepoMsg signals that a Git repository folder was selected.

type GitRepoMsg struct {
	Path string
}

// BackToFolderMsg signals that the user wants to leave the Git repo view.
type BackToFolderMsg struct{}
