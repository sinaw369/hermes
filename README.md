# Hermes

**Hermes** is a TUI (Text User Interface) tool that automates GitLab repository operations, such as cloning, pulling, and creating merge requests. With Hermes, developers can efficiently manage their Git workflows directly from the terminal.

## What is Hermes?

Inspired by the Greek god Hermes, the swift messenger of the gods, this tool streamlines GitLab repository management by automating repetitive tasks like cloning repositories, pulling updates, and creating merge requests. Hermes empowers developers to interact with GitLab repositories more efficiently, reducing manual effort and errors while allowing focus on core development tasks.

**Key advantages of Hermes**:
- **Automation**: Clone or pull GitLab repositories automatically using customizable patterns.
- **Merge Request Workflow**: Create feature branches, commit changes, and submit merge requests with minimal terminal commands.
- **Interactive TUI**: Track progress in real-time with logs, status updates, and an intuitive terminal interface.
- **Configurable**: Define include/exclude patterns and settings to tailor operations to your workflow.

## Features

- **Cloning & Pulling**: Automates cloning and pulling of repositories.
- **Merge Requests**: Creates branches, runs custom commands, commits, pushes, and creates merge requests.
- **Interactive Logs & Progress**: Real-time updates in a terminal UI.
- **Configurable**: Supports flexible include/exclude patterns and other settings.
## Installation
```bash
go install github.com/sinaw369/hermes@latest
```
### Notes
This command downloads the latest version of Hermes, builds it,
and installs the binary into your Go bin directory (typically `$GOPATH/bin` or `$HOME/go/bin`).
Make sure that your Go bin directory is in your system's PATH so you can run the hermes command from anywhere.
## Usage

create ```.env``` file
In the same directory as your compiled binary, create a file named `.env` with the following contents. Adjust the values as needed for your environment:
```.env
# Directory where pull requests will be downloaded
SYNC_DIR=path/to/download_pr_folder

# Directory to display in the UI for Git diff (the base folder for file browsing)
FILES_DIR=path/to/git_diff_folder

# Branches to use when showing a diff in the UI
DIFF_BRANCH_FROM=develop
DIFF_BRANCH_TO=production

# Synchronization interval (e.g., "1m" for one minute)
SYNC_INTERVAL=1m

# GitLab credentials and API base URL
GITLAB_TOKEN=your_gitlab_token_here
GITLAB_BASE_URL=https://gitlab.example.com


```
**Notes** :
* SYNC_DIR: This is where your application will download and sync pull request projects.
* FILES_DIR: This directory will be used by the UI to browse and display Git diff information.
* DIFF_BRANCH_FROM / DIFF_BRANCH_TO: These determine which two branches to compare when showing diffs.
* SYNC_INTERVAL: Specifies how frequently (e.g., every 1 minute) the sync process runs.
* GITLAB_TOKEN / GITLAB_BASE_URL: Provide your GitLab token and the base URL for your GitLab instance.