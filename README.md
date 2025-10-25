# GitHub Backup Tool (ghbackup)

ghbackup is a tool to backup repositories off your github profile and organizations. It will backup all branches off every repository it can access. The two commands are backup and login, and backup can take an argument for a folder to backup things. Your credentials are stored in ~/.ghbackup/.auth and ~/.ghbackup is the default backup folder.

When creating the personal access token, repo is required for the tool to work, but read:org is optional.