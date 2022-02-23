package keys

import _ "embed"

var (
	//go:embed armory-drive-log.pub
	ArmoryDriveLogPub string
	//go:embed armory-drive.pub
	ArmoryDrivePub string
)
