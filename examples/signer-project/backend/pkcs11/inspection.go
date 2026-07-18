package pkcs11backend

import (
	"errors"
	"fmt"

	"github.com/miekg/pkcs11"
)

// Inspection is vendor-neutral evidence returned by the selected PKCS#11
// module. It does not, by itself, prove that the module is backed by physical
// hardware; production acceptance must correlate these values with the
// vendor-native inventory, firmware, audit, and HA control planes.
type Inspection struct {
	Library   LibraryInfo       `json:"library"`
	Slot      SlotInfo          `json:"slot"`
	Token     TokenInfo         `json:"token"`
	Mechanism MechanismEvidence `json:"ecdsa_mechanism"`
	Key       KeyAttributes     `json:"private_key"`
	Warnings  []string          `json:"warnings,omitempty"`
}

type LibraryInfo struct {
	CryptokiVersion    string `json:"cryptoki_version"`
	ManufacturerID     string `json:"manufacturer_id"`
	LibraryDescription string `json:"library_description"`
	LibraryVersion     string `json:"library_version"`
}

type SlotInfo struct {
	ID              uint   `json:"id"`
	Description     string `json:"description"`
	ManufacturerID  string `json:"manufacturer_id"`
	HardwareSlot    bool   `json:"hardware_slot"`
	HardwareVersion string `json:"hardware_version"`
	FirmwareVersion string `json:"firmware_version"`
}

type TokenInfo struct {
	Label           string `json:"label"`
	ManufacturerID  string `json:"manufacturer_id"`
	Model           string `json:"model"`
	SerialNumber    string `json:"serial_number"`
	HardwareVersion string `json:"hardware_version"`
	FirmwareVersion string `json:"firmware_version"`
}

type MechanismEvidence struct {
	Name       string `json:"name"`
	Supported  bool   `json:"supported"`
	CanSign    bool   `json:"can_sign"`
	MinKeySize uint   `json:"min_key_size"`
	MaxKeySize uint   `json:"max_key_size"`
}

// KeyAttributes uses pointers because some HSMs deliberately hide or do not
// implement individual attributes. A nil value is "no evidence", not false.
type KeyAttributes struct {
	Token            *bool             `json:"token"`
	Private          *bool             `json:"private"`
	Sensitive        *bool             `json:"sensitive"`
	AlwaysSensitive  *bool             `json:"always_sensitive"`
	Extractable      *bool             `json:"extractable"`
	NeverExtractable *bool             `json:"never_extractable"`
	CanSign          *bool             `json:"can_sign"`
	Unavailable      map[string]string `json:"unavailable,omitempty"`
}

// ValidateSecureKey rejects dangerous attribute values. When requireEvidence
// is true it also rejects attributes the module did not expose.
func (i Inspection) ValidateSecureKey(requireEvidence bool) error {
	if !i.Mechanism.Supported || !i.Mechanism.CanSign {
		return errors.New("pkcs11 inspection: CKM_ECDSA signing is not supported")
	}
	checks := []struct {
		name string
		got  *bool
		want bool
	}{
		{"CKA_TOKEN", i.Key.Token, true},
		{"CKA_PRIVATE", i.Key.Private, true},
		{"CKA_SENSITIVE", i.Key.Sensitive, true},
		{"CKA_ALWAYS_SENSITIVE", i.Key.AlwaysSensitive, true},
		{"CKA_EXTRACTABLE", i.Key.Extractable, false},
		{"CKA_NEVER_EXTRACTABLE", i.Key.NeverExtractable, true},
		{"CKA_SIGN", i.Key.CanSign, true},
	}
	for _, check := range checks {
		if check.got == nil {
			if requireEvidence {
				return fmt.Errorf("pkcs11 inspection: %s was not readable", check.name)
			}
			continue
		}
		if *check.got != check.want {
			return fmt.Errorf(
				"pkcs11 inspection: %s=%t, want %t",
				check.name,
				*check.got,
				check.want,
			)
		}
	}
	return nil
}

// Inspect opens the module directly and gathers token, mechanism, and private
// key attribute evidence. Call it before Open so a module that maintains
// process-global PKCS#11 initialization state is not inspected concurrently
// with crypto11.
func Inspect(config Config) (Inspection, error) {
	if err := validateConfig(config); err != nil {
		return Inspection{}, err
	}
	ctx := pkcs11.New(config.ModulePath)
	if ctx == nil {
		return Inspection{}, errors.New("pkcs11 inspection: load module")
	}
	defer ctx.Destroy()

	initialized := false
	if err := ctx.Initialize(); err != nil {
		if !isPKCS11Error(err, pkcs11.CKR_CRYPTOKI_ALREADY_INITIALIZED) {
			return Inspection{}, fmt.Errorf("pkcs11 inspection: initialize: %w", err)
		}
	} else {
		initialized = true
	}
	if initialized {
		defer func() {
			_ = ctx.Finalize()
		}()
	}

	info, err := ctx.GetInfo()
	if err != nil {
		return Inspection{}, fmt.Errorf("pkcs11 inspection: get library info: %w", err)
	}
	slotID, token, err := selectSlot(ctx, config)
	if err != nil {
		return Inspection{}, err
	}
	slot, err := ctx.GetSlotInfo(slotID)
	if err != nil {
		return Inspection{}, fmt.Errorf("pkcs11 inspection: get slot info: %w", err)
	}
	report := Inspection{
		Library: LibraryInfo{
			CryptokiVersion:    formatVersion(info.CryptokiVersion),
			ManufacturerID:     info.ManufacturerID,
			LibraryDescription: info.LibraryDescription,
			LibraryVersion:     formatVersion(info.LibraryVersion),
		},
		Slot: SlotInfo{
			ID:              slotID,
			Description:     slot.SlotDescription,
			ManufacturerID:  slot.ManufacturerID,
			HardwareSlot:    slot.Flags&pkcs11.CKF_HW_SLOT != 0,
			HardwareVersion: formatVersion(slot.HardwareVersion),
			FirmwareVersion: formatVersion(slot.FirmwareVersion),
		},
		Token: TokenInfo{
			Label:           token.Label,
			ManufacturerID:  token.ManufacturerID,
			Model:           token.Model,
			SerialNumber:    token.SerialNumber,
			HardwareVersion: formatVersion(token.HardwareVersion),
			FirmwareVersion: formatVersion(token.FirmwareVersion),
		},
		Mechanism: MechanismEvidence{Name: "CKM_ECDSA"},
		Key: KeyAttributes{
			Unavailable: make(map[string]string),
		},
	}

	mechanismInfo, err := ctx.GetMechanismInfo(
		slotID,
		[]*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_ECDSA, nil)},
	)
	if err == nil {
		report.Mechanism.Supported = true
		report.Mechanism.CanSign = mechanismInfo.Flags&pkcs11.CKF_SIGN != 0
		report.Mechanism.MinKeySize = mechanismInfo.MinKeySize
		report.Mechanism.MaxKeySize = mechanismInfo.MaxKeySize
	} else if isPKCS11Error(err, pkcs11.CKR_MECHANISM_INVALID) {
		report.Warnings = append(report.Warnings, "CKM_ECDSA is not exposed by the selected token")
	} else {
		return Inspection{}, fmt.Errorf("pkcs11 inspection: get CKM_ECDSA info: %w", err)
	}

	session, err := ctx.OpenSession(slotID, pkcs11.CKF_SERIAL_SESSION|pkcs11.CKF_RW_SESSION)
	if err != nil {
		return Inspection{}, fmt.Errorf("pkcs11 inspection: open session: %w", err)
	}
	defer func() {
		_ = ctx.CloseSession(session)
	}()
	loggedIn := false
	if err = ctx.Login(session, pkcs11.CKU_USER, config.PIN); err != nil {
		if !isPKCS11Error(err, pkcs11.CKR_USER_ALREADY_LOGGED_IN) {
			return Inspection{}, fmt.Errorf("pkcs11 inspection: login: %w", err)
		}
	} else {
		loggedIn = true
	}
	if loggedIn {
		defer func() {
			_ = ctx.Logout(session)
		}()
	}

	key, err := findPrivateKey(ctx, session, config)
	if err != nil {
		return Inspection{}, err
	}
	attributes := []struct {
		name   string
		kind   uint
		target **bool
	}{
		{"CKA_TOKEN", pkcs11.CKA_TOKEN, &report.Key.Token},
		{"CKA_PRIVATE", pkcs11.CKA_PRIVATE, &report.Key.Private},
		{"CKA_SENSITIVE", pkcs11.CKA_SENSITIVE, &report.Key.Sensitive},
		{"CKA_ALWAYS_SENSITIVE", pkcs11.CKA_ALWAYS_SENSITIVE, &report.Key.AlwaysSensitive},
		{"CKA_EXTRACTABLE", pkcs11.CKA_EXTRACTABLE, &report.Key.Extractable},
		{"CKA_NEVER_EXTRACTABLE", pkcs11.CKA_NEVER_EXTRACTABLE, &report.Key.NeverExtractable},
		{"CKA_SIGN", pkcs11.CKA_SIGN, &report.Key.CanSign},
	}
	for _, attribute := range attributes {
		value, err := readBooleanAttribute(ctx, session, key, attribute.kind)
		if err != nil {
			report.Key.Unavailable[attribute.name] = err.Error()
			continue
		}
		*attribute.target = &value
	}
	if len(report.Key.Unavailable) == 0 {
		report.Key.Unavailable = nil
	}
	if !report.Slot.HardwareSlot {
		report.Warnings = append(
			report.Warnings,
			"CKF_HW_SLOT is false; correlate the token with the vendor-native hardware inventory",
		)
	}
	return report, nil
}

func selectSlot(ctx *pkcs11.Ctx, config Config) (uint, pkcs11.TokenInfo, error) {
	slots, err := ctx.GetSlotList(true)
	if err != nil {
		return 0, pkcs11.TokenInfo{}, fmt.Errorf("pkcs11 inspection: list slots: %w", err)
	}
	type match struct {
		id    uint
		token pkcs11.TokenInfo
	}
	matches := make([]match, 0, 1)
	for _, slotID := range slots {
		token, err := ctx.GetTokenInfo(slotID)
		if err != nil {
			return 0, pkcs11.TokenInfo{}, fmt.Errorf(
				"pkcs11 inspection: get token info for slot %d: %w",
				slotID,
				err,
			)
		}
		selected := false
		switch {
		case config.TokenLabel != "":
			selected = token.Label == config.TokenLabel
		case config.TokenSerial != "":
			selected = token.SerialNumber == config.TokenSerial
		case config.SlotNumber != nil:
			selected = slotID == uint(*config.SlotNumber)
		}
		if selected {
			matches = append(matches, match{id: slotID, token: token})
		}
	}
	if len(matches) != 1 {
		return 0, pkcs11.TokenInfo{}, fmt.Errorf(
			"pkcs11 inspection: token selector matched %d slots",
			len(matches),
		)
	}
	return matches[0].id, matches[0].token, nil
}

func findPrivateKey(
	ctx *pkcs11.Ctx,
	session pkcs11.SessionHandle,
	config Config,
) (pkcs11.ObjectHandle, error) {
	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PRIVATE_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_KEY_TYPE, pkcs11.CKK_EC),
		pkcs11.NewAttribute(pkcs11.CKA_ID, config.ObjectID),
		pkcs11.NewAttribute(pkcs11.CKA_LABEL, config.ObjectLabel),
	}
	if err := ctx.FindObjectsInit(session, template); err != nil {
		return 0, fmt.Errorf("pkcs11 inspection: initialize key search: %w", err)
	}
	objects, more, err := ctx.FindObjects(session, 2)
	finalErr := ctx.FindObjectsFinal(session)
	if err != nil {
		return 0, fmt.Errorf("pkcs11 inspection: find private key: %w", err)
	}
	if finalErr != nil {
		return 0, fmt.Errorf("pkcs11 inspection: finalize key search: %w", finalErr)
	}
	switch {
	case len(objects) == 0:
		return 0, fmt.Errorf(
			"%w: private CKA_ID=%x CKA_LABEL=%q",
			ErrKeyNotFound,
			config.ObjectID,
			config.ObjectLabel,
		)
	case len(objects) != 1 || more:
		return 0, fmt.Errorf(
			"%w: private CKA_ID=%x CKA_LABEL=%q",
			ErrKeyAmbiguous,
			config.ObjectID,
			config.ObjectLabel,
		)
	default:
		return objects[0], nil
	}
}

func readBooleanAttribute(
	ctx *pkcs11.Ctx,
	session pkcs11.SessionHandle,
	key pkcs11.ObjectHandle,
	kind uint,
) (bool, error) {
	values, err := ctx.GetAttributeValue(
		session,
		key,
		[]*pkcs11.Attribute{pkcs11.NewAttribute(kind, nil)},
	)
	if err != nil {
		return false, err
	}
	if len(values) != 1 || len(values[0].Value) != 1 {
		return false, fmt.Errorf("unexpected boolean encoding length %d", attributeLength(values))
	}
	return values[0].Value[0] != 0, nil
}

func attributeLength(attributes []*pkcs11.Attribute) int {
	if len(attributes) != 1 || attributes[0] == nil {
		return -1
	}
	return len(attributes[0].Value)
}

func formatVersion(version pkcs11.Version) string {
	return fmt.Sprintf("%d.%d", version.Major, version.Minor)
}

func isPKCS11Error(err error, target pkcs11.Error) bool {
	var actual pkcs11.Error
	return errors.As(err, &actual) && actual == target
}
