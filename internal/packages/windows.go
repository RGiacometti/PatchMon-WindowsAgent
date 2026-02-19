package packages

import (
	"fmt"
	"runtime"
	"strings"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/sirupsen/logrus"

	"patchmon-agent/pkg/models"
)

// WindowsUpdateManager handles Windows Update COM API interactions
type WindowsUpdateManager struct {
	logger *logrus.Logger
}

// NewWindowsUpdateManager creates a new WindowsUpdateManager
func NewWindowsUpdateManager(logger *logrus.Logger) *WindowsUpdateManager {
	return &WindowsUpdateManager{logger: logger}
}

// GetInstalledUpdates returns all installed Windows updates
func (w *WindowsUpdateManager) GetInstalledUpdates() ([]models.Package, error) {
	return w.searchUpdates("IsInstalled=1")
}

// GetAvailableUpdates returns all available (not installed, not hidden) updates
func (w *WindowsUpdateManager) GetAvailableUpdates() ([]models.Package, error) {
	w.logger.Info("Searching for available Windows updates (this may take 30-60 seconds)...")
	return w.searchUpdates("IsInstalled=0 AND IsHidden=0")
}

// searchUpdates queries the Windows Update Agent COM API with the given search criteria
func (w *WindowsUpdateManager) searchUpdates(criteria string) ([]models.Package, error) {
	// COM must be initialized on the same OS thread
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	if err != nil {
		// S_FALSE (0x00000001) means COM is already initialized on this thread — that's OK
		oleErr, ok := err.(*ole.OleError)
		if !ok || oleErr.Code() != 0x00000001 {
			return nil, fmt.Errorf("COM initialization failed: %w", err)
		}
	}
	defer ole.CoUninitialize()

	// Create Microsoft.Update.Session
	unknown, err := oleutil.CreateObject("Microsoft.Update.Session")
	if err != nil {
		return nil, fmt.Errorf("failed to create UpdateSession: %w", err)
	}
	defer unknown.Release()

	session, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, fmt.Errorf("failed to query UpdateSession interface: %w", err)
	}
	defer session.Release()

	// Create UpdateSearcher via session.CreateUpdateSearcher()
	searcherResult, err := oleutil.CallMethod(session, "CreateUpdateSearcher")
	if err != nil {
		return nil, fmt.Errorf("failed to create UpdateSearcher: %w", err)
	}
	searcher := searcherResult.ToIDispatch()
	defer searcher.Release()

	// Search for updates matching the criteria
	w.logger.Debugf("Searching Windows Updates with criteria: %s", criteria)
	resultVal, err := oleutil.CallMethod(searcher, "Search", criteria)
	if err != nil {
		return nil, fmt.Errorf("update search failed (criteria=%q): %w", criteria, err)
	}
	result := resultVal.ToIDispatch()
	defer result.Release()

	// Get the Updates collection from the search result
	updatesVal, err := oleutil.GetProperty(result, "Updates")
	if err != nil {
		return nil, fmt.Errorf("failed to get Updates collection: %w", err)
	}
	updates := updatesVal.ToIDispatch()
	defer updates.Release()

	// Get the count of updates
	countVal, err := oleutil.GetProperty(updates, "Count")
	if err != nil {
		return nil, fmt.Errorf("failed to get update count: %w", err)
	}
	count := int(countVal.Val)

	w.logger.Debugf("Found %d updates for criteria: %s", count, criteria)

	packages := make([]models.Package, 0, count)

	for i := 0; i < count; i++ {
		itemVal, err := oleutil.GetProperty(updates, "Item", i)
		if err != nil {
			w.logger.Warnf("Failed to get update item %d: %v", i, err)
			continue
		}
		update := itemVal.ToIDispatch()

		pkg := w.parseUpdate(update, criteria)
		if pkg != nil {
			packages = append(packages, *pkg)
		}

		update.Release()
	}

	return packages, nil
}

// parseUpdate extracts package information from a single IUpdate COM object
func (w *WindowsUpdateManager) parseUpdate(update *ole.IDispatch, criteria string) *models.Package {
	// Get Title
	titleVal, err := oleutil.GetProperty(update, "Title")
	if err != nil {
		w.logger.Warn("Failed to get update title")
		return nil
	}
	title := titleVal.ToString()

	// Get KB Article ID
	kbID := w.getKBArticleID(update)

	// Get Identity for version info
	version := w.getUpdateVersion(update)

	// Check if this is a security update
	isSecurityUpdate := w.isSecurityUpdate(update)

	// Determine name: use KB ID if available, otherwise use title
	name := title
	if kbID != "" {
		name = "KB" + kbID
	}

	// Determine if this is an installed or available update based on search criteria
	isInstalled := strings.Contains(criteria, "IsInstalled=1")

	pkg := &models.Package{
		Name:             name,
		Description:      title,
		NeedsUpdate:      !isInstalled,
		IsSecurityUpdate: isSecurityUpdate,
	}

	if isInstalled {
		pkg.CurrentVersion = version
	} else {
		pkg.AvailableVersion = version
	}

	return pkg
}

// getKBArticleID extracts the first KB article ID from an update
func (w *WindowsUpdateManager) getKBArticleID(update *ole.IDispatch) string {
	kbIDsVal, err := oleutil.GetProperty(update, "KBArticleIDs")
	if err != nil {
		return ""
	}
	kbIDs := kbIDsVal.ToIDispatch()
	defer kbIDs.Release()

	countVal, err := oleutil.GetProperty(kbIDs, "Count")
	if err != nil || countVal.Val == 0 {
		return ""
	}

	itemVal, err := oleutil.GetProperty(kbIDs, "Item", 0)
	if err != nil {
		return ""
	}
	return itemVal.ToString()
}

// getUpdateVersion extracts version information from the update's Identity
func (w *WindowsUpdateManager) getUpdateVersion(update *ole.IDispatch) string {
	identityVal, err := oleutil.GetProperty(update, "Identity")
	if err != nil {
		return ""
	}
	identity := identityVal.ToIDispatch()
	defer identity.Release()

	revVal, err := oleutil.GetProperty(identity, "RevisionNumber")
	if err != nil {
		return ""
	}

	updateIDVal, err := oleutil.GetProperty(identity, "UpdateID")
	if err != nil {
		return fmt.Sprintf("rev.%d", revVal.Val)
	}

	return fmt.Sprintf("%s.%d", updateIDVal.ToString(), revVal.Val)
}

// isSecurityUpdate determines if an update is security-related by checking
// MsrcSeverity and Categories
func (w *WindowsUpdateManager) isSecurityUpdate(update *ole.IDispatch) bool {
	// Check MsrcSeverity first — if it has a value, it's a security update
	severityVal, err := oleutil.GetProperty(update, "MsrcSeverity")
	if err == nil && severityVal.ToString() != "" {
		return true
	}

	// Check Categories for "Security Updates" or "Critical Updates"
	categoriesVal, err := oleutil.GetProperty(update, "Categories")
	if err != nil {
		return false
	}
	categories := categoriesVal.ToIDispatch()
	defer categories.Release()

	countVal, err := oleutil.GetProperty(categories, "Count")
	if err != nil {
		return false
	}
	count := int(countVal.Val)

	for i := 0; i < count; i++ {
		catVal, err := oleutil.GetProperty(categories, "Item", i)
		if err != nil {
			continue
		}
		cat := catVal.ToIDispatch()

		nameVal, err := oleutil.GetProperty(cat, "Name")
		cat.Release()
		if err != nil {
			continue
		}

		catName := nameVal.ToString()
		if catName == "Security Updates" || catName == "Critical Updates" {
			return true
		}
	}

	return false
}
