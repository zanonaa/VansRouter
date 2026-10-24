// Package database — migração do volume legado (pré-0.91.22) no boot.
// Porta fiel de docker/migrate-legacy-volume.cjs (Fase 6): cópia sem
// sobrescrever, validação de integridade SQLite, lock com detecção de dono
// morto e marker atômico com fsync. Executada antes de abrir o banco.
package database

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

const legacyMigrationSourceEnv = "MIGRATION_DATA_DIR"

type legacyLockOwner struct {
	Pid       int    `json:"pid"`
	Token     string `json:"token"`
	StartedAt string `json:"startedAt"`
}

func legacyValidateSQLite(filePath string) bool {
	if _, err := os.Stat(filePath); err != nil {
		return false
	}
	db, err := sql.Open("sqlite", "file:"+filePath+"?mode=ro")
	if err != nil {
		return false
	}
	defer db.Close()
	var result string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		return false
	}
	return result == "ok"
}

func legacySyncFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	_ = file.Sync()
	file.Close()
}

func legacySyncDirectory(dir string) {
	entry, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = entry.Sync()
	entry.Close()
}

// copyMissing copia sem sobrescrever, pulando o banco canônico e seus
// sidecars (o banco principal é movido explicitamente depois da validação).
// relative acumula o caminho a partir da raiz da cópia, como no original.
func legacyCopyMissing(source, target string) error {
	return legacyCopyMissingRelative(source, target, "")
}

func legacyCopyMissingRelative(source, target, relative string) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(source, entry.Name())
		targetPath := filepath.Join(target, entry.Name())
		relativePath := filepath.ToSlash(filepath.Join(relative, entry.Name()))
		if entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			if err := legacyCopyMissingRelative(sourcePath, targetPath, relativePath); err != nil {
				return err
			}
			continue
		}
		if relativePath == "db/data.sqlite" || (len(relativePath) > len("db/data.sqlite-") && relativePath[:len("db/data.sqlite-")] == "db/data.sqlite-") {
			continue
		}
		if _, err := os.Lstat(targetPath); err == nil {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(sourcePath)
			if err != nil {
				return err
			}
			if err := os.Symlink(link, targetPath); err != nil {
				return err
			}
			continue
		}
		if err := legacyCopyFile(sourcePath, targetPath); err != nil {
			return err
		}
	}
	return nil
}

func legacyCopyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// copyTree copia a árvore inteira (staging), incluindo o banco, com
// sobrescrita desativada e symlinks preservados.
func legacyCopyTree(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return os.MkdirAll(target, 0o755)
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, destination)
		}
		return legacyCopyFile(path, destination)
	})
}

func legacyWriteMarker(markerPath string) error {
	temporary := fmt.Sprintf("%s.tmp-%d-%d", markerPath, os.Getpid(), time.Now().UnixNano())
	if err := os.WriteFile(temporary, []byte("migrated\n"), 0o600); err != nil {
		return err
	}
	legacySyncFile(temporary)
	if err := os.Rename(temporary, markerPath); err != nil {
		return err
	}
	legacySyncDirectory(filepath.Dir(markerPath))
	return nil
}

func legacyCleanStaging(dataDir string) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() && len(entry.Name()) > len(".legacy-migration-") && entry.Name()[:len(".legacy-migration-")] == ".legacy-migration-" {
			_ = os.RemoveAll(filepath.Join(dataDir, entry.Name()))
		}
	}
}

// true = dono vivo; false = dono morto; nil = indeterminado.
func legacyLockOwnerAlive(lockPath string) *bool {
	raw, err := os.ReadFile(filepath.Join(lockPath, "owner.json"))
	if err != nil {
		return nil
	}
	var owner legacyLockOwner
	if json.Unmarshal(raw, &owner) != nil || owner.Pid <= 0 {
		return nil
	}
	err = syscall.Kill(owner.Pid, 0)
	if err == nil {
		alive := true
		return &alive
	}
	if errors.Is(err, syscall.ESRCH) {
		alive := false
		return &alive
	}
	return nil
}

func legacyAcquireLock(dataDir string, stale time.Duration) (func(), error) {
	lockPath := filepath.Join(dataDir, ".legacy-migration.lock")
	token := fmt.Sprintf("%d-%d-%s", os.Getpid(), time.Now().UnixNano(), strconv.FormatInt(time.Now().UnixNano(), 36))
	create := func() error {
		if err := os.Mkdir(lockPath, 0o755); err != nil {
			return err
		}
		owner, _ := json.Marshal(legacyLockOwner{Pid: os.Getpid(), Token: token, StartedAt: time.Now().UTC().Format(time.RFC3339Nano)})
		return os.WriteFile(filepath.Join(lockPath, "owner.json"), owner, 0o600)
	}
	if err := create(); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		info, statErr := os.Stat(lockPath)
		staleDetected := statErr == nil && time.Since(info.ModTime()) > stale
		if alive := legacyLockOwnerAlive(lockPath); alive != nil && !*alive {
			staleDetected = true
		}
		if raw, readErr := os.ReadFile(filepath.Join(lockPath, "owner.json")); readErr == nil {
			var owner legacyLockOwner
			if json.Unmarshal(raw, &owner) != nil || owner.Pid <= 0 {
				if statErr == nil && time.Since(info.ModTime()) > 30*time.Second {
					staleDetected = true
				}
			}
		} else if statErr == nil && time.Since(info.ModTime()) > 30*time.Second {
			staleDetected = true
		}
		if !staleDetected {
			return nil, fmt.Errorf("Legacy migration is already running for %s", dataDir)
		}
		_ = os.RemoveAll(lockPath)
		if err := create(); err != nil {
			return nil, err
		}
	}
	release := func() {
		raw, err := os.ReadFile(filepath.Join(lockPath, "owner.json"))
		if err != nil {
			return
		}
		var owner legacyLockOwner
		if json.Unmarshal(raw, &owner) != nil || owner.Token != token {
			return
		}
		_ = os.RemoveAll(lockPath)
	}
	return release, nil
}

// MigrateLegacyVolume porta docker/migrate-legacy-volume.cjs: move o volume
// legado para o DATA_DIR antes de abrir o banco. Fonte padrão /migration-data
// (MIGRATION_DATA_DIR). Mensagens de erro idênticas às do script original.
func MigrateLegacyVolume(dataDir string) (string, error) {
	if dataDir == "" {
		dataDir = os.Getenv("DATA_DIR")
	}
	if dataDir == "" {
		dataDir = "/app/data"
	}
	migrationDir := os.Getenv(legacyMigrationSourceEnv)
	if migrationDir == "" {
		migrationDir = "/migration-data"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", err
	}
	release, err := legacyAcquireLock(dataDir, 6*time.Hour)
	if err != nil {
		return "", err
	}
	defer release()

	dbDir := filepath.Join(dataDir, "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		return "", err
	}
	legacyCleanStaging(dataDir)

	destinationDb := filepath.Join(dbDir, "data.sqlite")
	markerPath := filepath.Join(dbDir, ".legacy-volume-migrated")
	destinationExists := fileExists(destinationDb)
	markerExists := fileExists(markerPath)

	if markerExists && destinationExists {
		if fileExists(migrationDir) {
			if err := legacyCopyMissing(migrationDir, dataDir); err != nil {
				return destinationDb, err
			}
		}
		return destinationDb, nil
	}

	destinationValid := destinationExists && legacyValidateSQLite(destinationDb)
	if markerExists && !destinationValid {
		_ = os.Remove(markerPath)
	}
	if !fileExists(migrationDir) {
		if destinationExists && !destinationValid {
			return destinationDb, fmt.Errorf("Canonical database is invalid and no migration source exists: %s", destinationDb)
		}
		if err := legacyWriteMarker(markerPath); err != nil {
			return destinationDb, err
		}
		return destinationDb, nil
	}

	staging := filepath.Join(dataDir, fmt.Sprintf(".legacy-migration-%d-%d", os.Getpid(), time.Now().UnixNano()))
	if err := legacyCopyTree(migrationDir, staging); err != nil {
		return destinationDb, err
	}
	defer os.RemoveAll(staging)

	stagedDb := filepath.Join(staging, "db", "data.sqlite")
	stagedExists := fileExists(stagedDb)
	if !destinationValid {
		for _, suffix := range []string{"-wal", "-shm"} {
			if fileExists(stagedDb + suffix) {
				return destinationDb, fmt.Errorf("Migration source contains an active SQLite sidecar (data.sqlite%s); checkpoint it before migration", suffix)
			}
		}
		if stagedExists && !legacyValidateSQLite(stagedDb) {
			return destinationDb, fmt.Errorf("Migration source contains an invalid SQLite database: %s", stagedDb)
		}
	}

	if err := legacyCopyMissing(staging, dataDir); err != nil {
		return destinationDb, err
	}

	if destinationValid {
		if err := legacyWriteMarker(markerPath); err != nil {
			return destinationDb, err
		}
		return destinationDb, nil
	}
	if stagedExists {
		if destinationExists {
			backup := fmt.Sprintf("%s.invalid-%d-%d", destinationDb, time.Now().UnixNano(), os.Getpid())
			if err := os.Rename(destinationDb, backup); err != nil {
				return destinationDb, err
			}
			for _, suffix := range []string{"-wal", "-shm"} {
				if fileExists(destinationDb + suffix) {
					_ = os.Rename(destinationDb+suffix, backup+suffix)
				}
			}
		}
		if err := os.MkdirAll(filepath.Dir(destinationDb), 0o755); err != nil {
			return destinationDb, err
		}
		if err := os.Rename(stagedDb, destinationDb); err != nil {
			return destinationDb, err
		}
		legacySyncDirectory(filepath.Dir(destinationDb))
		if !legacyValidateSQLite(destinationDb) {
			return destinationDb, fmt.Errorf("Migrated database failed validation: %s", destinationDb)
		}
		if err := legacyWriteMarker(markerPath); err != nil {
			return destinationDb, err
		}
		return destinationDb, nil
	}
	if destinationExists {
		return destinationDb, fmt.Errorf("Canonical database is invalid and migration source has no SQLite database: %s", destinationDb)
	}
	if err := legacyWriteMarker(markerPath); err != nil {
		return destinationDb, err
	}
	return destinationDb, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
