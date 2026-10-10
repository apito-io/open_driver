package sqlquery

import (
	"context"
	"fmt"
	"sort"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/relationmigrate"
	"github.com/apito-io/engine/utility"
	"github.com/uptrace/bun"
	"gitlab.com/apito.io/open_driver/project/sqlcommon"
)

// SQLSession is the dialect-neutral executor used by every SQL project driver.
type SQLSession struct {
	Engine     string
	ScopeKey   string
	Exec       func(query string) error
	ExecArgs   func(query string, args ...interface{}) error
	Query      func(query string) ([][]string, error)
	DropColumn func(table, column string) error
}

type fkSpec struct {
	Holder string
	Column string
	Ref    string
	Unique bool
}

type storageSpec struct {
	Label      string
	FKs        []fkSpec
	Pivot      string
	PivotLeft  string
	PivotRight string
}

func describeStorage(fromModel, fromRel, toModel, toRel, knownAs string) (storageSpec, error) {
	fromRel = strings.ToLower(strings.TrimSpace(fromRel))
	toRel = strings.ToLower(strings.TrimSpace(toRel))
	fromTable := utility.PhysicalSQLTableName(fromModel)
	toTable := utility.PhysicalSQLTableName(toModel)
	if fromTable == "" || toTable == "" {
		return storageSpec{}, fmt.Errorf("relation models are required")
	}
	known := strings.TrimSpace(knownAs)
	colOn := func(holder, peer string, unique bool) fkSpec {
		return fkSpec{Holder: holder, Column: relationFKColumnNameParts(peer, known), Ref: peer, Unique: unique}
	}
	switch relationmigrate.ShapeOf(fromRel, toRel) {
	case relationmigrate.ShapeManyToMany:
		pivot := relationPivotTableNameParts(fromModel, toModel, known)
		left, right := sortedPhysicalPair(fromModel, toModel)
		return storageSpec{Label: "pivot " + pivot, Pivot: pivot, PivotLeft: left, PivotRight: right}, nil
	case relationmigrate.ShapeOneToOne:
		return storageSpec{
			Label: fmt.Sprintf("FK %s.%s and %s.%s", fromTable, relationFKColumnNameParts(toTable, known), toTable, relationFKColumnNameParts(fromTable, known)),
			FKs:   []fkSpec{colOn(fromTable, toTable, true), colOn(toTable, fromTable, true)},
		}, nil
	case relationmigrate.ShapeOneToMany:
		// from has_many, to has_one: FK lives on to.
		fk := colOn(toTable, fromTable, false)
		return storageSpec{Label: fmt.Sprintf("FK %s.%s", fk.Holder, fk.Column), FKs: []fkSpec{fk}}, nil
	case relationmigrate.ShapeManyToOne:
		fk := colOn(fromTable, toTable, false)
		return storageSpec{Label: fmt.Sprintf("FK %s.%s", fk.Holder, fk.Column), FKs: []fkSpec{fk}}, nil
	default:
		return storageSpec{}, fmt.Errorf("unsupported relation shape %s/%s", fromRel, toRel)
	}
}

type inspected struct {
	spec          storageSpec
	present       bool
	pairs         []relationmigrate.Pair
	dangling      int
	contradictory int
}

func (s SQLSession) quote(ident string) string {
	if isPostgresEngine(s.Engine) {
		return sqlcommon.QuotePGIdent(ident)
	}
	return sqlcommon.QuoteMySQLIdent(ident)
}

func isPostgresEngine(engine string) bool {
	return strings.EqualFold(strings.TrimSpace(engine), _const.PostgreSQLDriver)
}

func (s SQLSession) tableExists(table string) (bool, error) {
	var q string
	switch {
	case sqlcommon.EngineIsSQLiteLike(s.Engine):
		q = fmt.Sprintf(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = '%s'`, strings.ReplaceAll(table, "'", ""))
	case isPostgresEngine(s.Engine):
		q = fmt.Sprintf(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = '%s'`, strings.ReplaceAll(table, "'", ""))
	default:
		q = fmt.Sprintf(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = '%s'`, strings.ReplaceAll(table, "'", ""))
	}
	rows, err := s.Query(q)
	if err != nil {
		return false, err
	}
	return len(rows) > 0 && rows[0][0] != "0", nil
}

func (s SQLSession) columnExists(table, column string) (bool, error) {
	ok, err := s.tableExists(table)
	if err != nil || !ok {
		return false, err
	}
	var q string
	switch {
	case sqlcommon.EngineIsSQLiteLike(s.Engine):
		q = fmt.Sprintf(`SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = '%s'`, table, column)
	case isPostgresEngine(s.Engine):
		q = fmt.Sprintf(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = '%s' AND column_name = '%s'`, table, column)
	default:
		q = fmt.Sprintf(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = '%s' AND column_name = '%s'`, table, column)
	}
	rows, err := s.Query(q)
	if err != nil {
		return false, err
	}
	return len(rows) > 0 && rows[0][0] != "0", nil
}

func canonicalPair(modelA, idA, modelB, idB string) relationmigrate.Pair {
	a := utility.PhysicalSQLTableName(modelA)
	b := utility.PhysicalSQLTableName(modelB)
	if a <= b {
		return relationmigrate.Pair{Left: idA, Right: idB}
	}
	return relationmigrate.Pair{Left: idB, Right: idA}
}

func (s SQLSession) inspect(fromModel, fromRel, toModel, toRel, knownAs string) (inspected, error) {
	spec, err := describeStorage(fromModel, fromRel, toModel, toRel, knownAs)
	if err != nil {
		return inspected{}, err
	}
	out := inspected{spec: spec}
	if spec.Pivot != "" {
		ok, err := s.tableExists(spec.Pivot)
		if err != nil {
			return out, err
		}
		out.present = ok
		if !ok {
			return out, nil
		}
		q := fmt.Sprintf(`SELECT %s, %s FROM %s`, s.quote(spec.PivotLeft+"_id"), s.quote(spec.PivotRight+"_id"), s.quote(spec.Pivot))
		rows, err := s.Query(q)
		if err != nil {
			return out, err
		}
		leftTable, rightTable := spec.PivotLeft, spec.PivotRight
		leftIDs, err := s.idSet(leftTable)
		if err != nil {
			return out, err
		}
		rightIDs, err := s.idSet(rightTable)
		if err != nil {
			return out, err
		}
		for _, row := range rows {
			if len(row) < 2 || row[0] == "" || row[1] == "" {
				continue
			}
			if _, ok := leftIDs[row[0]]; !ok {
				out.dangling++
				continue
			}
			if _, ok := rightIDs[row[1]]; !ok {
				out.dangling++
				continue
			}
			out.pairs = append(out.pairs, relationmigrate.Pair{Left: row[0], Right: row[1]})
		}
		return out, nil
	}
	var pairSets [][]relationmigrate.Pair
	any := false
	for _, fk := range spec.FKs {
		ok, err := s.columnExists(fk.Holder, fk.Column)
		if err != nil {
			return out, err
		}
		if !ok {
			continue
		}
		any = true
		rows, err := s.Query(fmt.Sprintf(`SELECT %s, %s FROM %s WHERE %s IS NOT NULL AND %s != ''`,
			s.quote("id"), s.quote(fk.Column), s.quote(fk.Holder), s.quote(fk.Column), s.quote(fk.Column)))
		if err != nil {
			return out, err
		}
		refIDs, err := s.idSet(fk.Ref)
		if err != nil {
			return out, err
		}
		var pairs []relationmigrate.Pair
		for _, row := range rows {
			if len(row) < 2 || row[0] == "" || row[1] == "" {
				continue
			}
			if _, ok := refIDs[row[1]]; !ok {
				out.dangling++
				continue
			}
			pairs = append(pairs, canonicalPair(fk.Holder, row[0], fk.Ref, row[1]))
		}
		pairSets = append(pairSets, pairs)
	}
	out.present = any
	if len(pairSets) == 2 {
		out.pairs, out.contradictory = unionDual(pairSets[0], pairSets[1])
	} else if len(pairSets) == 1 {
		out.pairs = uniquePairs(pairSets[0])
	}
	return out, nil
}

func (s SQLSession) idSet(table string) (map[string]struct{}, error) {
	ok, err := s.tableExists(table)
	if err != nil || !ok {
		return map[string]struct{}{}, err
	}
	rows, err := s.Query(fmt.Sprintf(`SELECT %s FROM %s`, s.quote("id"), s.quote(table)))
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if len(row) > 0 && row[0] != "" {
			out[row[0]] = struct{}{}
		}
	}
	return out, nil
}

func unionDual(a, b []relationmigrate.Pair) ([]relationmigrate.Pair, int) {
	type key struct{ l, r string }
	seen := map[key]struct{}{}
	var out []relationmigrate.Pair
	add := func(p relationmigrate.Pair) {
		k := key{p.Left, p.Right}
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		out = append(out, p)
	}
	for _, p := range a {
		add(p)
	}
	for _, p := range b {
		add(p)
	}
	// Contradiction: the same left id pairs with two rights, or one right points back differently.
	leftPartner := map[string]string{}
	rightPartner := map[string]string{}
	contra := 0
	for _, p := range a {
		if prev, ok := leftPartner[p.Left]; ok && prev != p.Right {
			contra++
		}
		leftPartner[p.Left] = p.Right
		if prev, ok := rightPartner[p.Right]; ok && prev != p.Left {
			contra++
		} else {
			rightPartner[p.Right] = p.Left
		}
	}
	for _, p := range b {
		if prev, ok := rightPartner[p.Right]; ok && prev != p.Left {
			contra++
		}
		if prev, ok := leftPartner[p.Left]; ok && prev != p.Right {
			contra++
		}
	}
	return out, contra
}

func uniquePairs(in []relationmigrate.Pair) []relationmigrate.Pair {
	type key struct{ l, r string }
	seen := map[key]struct{}{}
	var out []relationmigrate.Pair
	for _, p := range in {
		k := key{p.Left, p.Right}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Left == out[j].Left {
			return out[i].Right < out[j].Right
		}
		return out[i].Left < out[j].Left
	})
	return out
}

func degreeConflicts(pairs []relationmigrate.Pair, fromModel, fromRel, toModel, toRel string) int {
	shape := relationmigrate.ShapeOf(fromRel, toRel)
	if shape == relationmigrate.ShapeManyToMany {
		return 0
	}
	fromTable := utility.PhysicalSQLTableName(fromModel)
	toTable := utility.PhysicalSQLTableName(toModel)
	left, _ := sortedPhysicalPair(fromModel, toModel)
	count := func(side string) int {
		partners := map[string]map[string]struct{}{}
		for _, p := range pairs {
			id, other := p.Left, p.Right
			if side != left {
				id, other = p.Right, p.Left
			}
			if partners[id] == nil {
				partners[id] = map[string]struct{}{}
			}
			partners[id][other] = struct{}{}
		}
		n := 0
		for _, set := range partners {
			if len(set) > 1 {
				n++
			}
		}
		return n
	}
	switch shape {
	case relationmigrate.ShapeOneToOne:
		return count(fromTable) + count(toTable)
	case relationmigrate.ShapeOneToMany:
		return count(toTable)
	case relationmigrate.ShapeManyToOne:
		return count(fromTable)
	default:
		return 0
	}
}

func pairSetEqual(a, b []relationmigrate.Pair) bool {
	aa, bb := uniquePairs(a), uniquePairs(b)
	if len(aa) != len(bb) {
		return false
	}
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}

// PreviewRelationCardinality reads storage and returns the shared preview. It does not write.
func PreviewRelationCardinality(session SQLSession, req relationmigrate.Request) (relationmigrate.Preview, error) {
	req = req.Normalized()
	src, err := session.inspect(req.FromModel, req.FromRelation, req.ToModel, req.ToRelation, req.KnownAs)
	if err != nil {
		return relationmigrate.Preview{}, err
	}
	dst, err := describeStorage(req.FromModel, req.TargetFromRelation, req.ToModel, req.TargetToRelation, req.KnownAs)
	if err != nil {
		return relationmigrate.Preview{}, err
	}
	pairs := src.pairs
	if !src.present {
		// Already migrated: count the target if it exists.
		have, herr := session.inspect(req.FromModel, req.TargetFromRelation, req.ToModel, req.TargetToRelation, req.KnownAs)
		if herr != nil {
			return relationmigrate.Preview{}, herr
		}
		if have.present {
			pairs = have.pairs
			src.dangling = have.dangling
		}
	}
	conflicts := degreeConflicts(pairs, req.FromModel, req.TargetFromRelation, req.ToModel, req.TargetToRelation)
	snap := relationmigrate.DatabaseSnapshot{
		ScopeKey: session.ScopeKey, Capability: relationmigrate.CapabilitySQL,
		Counts: relationmigrate.Counts{
			SourcePairs: len(uniquePairs(pairs)) + src.dangling, ValidPairs: len(uniquePairs(pairs)), Dangling: src.dangling,
			Conflicts: conflicts, Contradictory: src.contradictory, ProjectedDestination: len(uniquePairs(pairs)),
		},
	}
	p := relationmigrate.BuildPreview(req, src.spec.Label, dst.Label, []relationmigrate.DatabaseSnapshot{snap})
	return p, nil
}

// MigrateRelationCardinality copies valid pairs, verifies the sets, then drops obsolete storage.
func MigrateRelationCardinality(session SQLSession, req relationmigrate.Request) (relationmigrate.Receipt, error) {
	preview, err := PreviewRelationCardinality(session, req)
	if err != nil {
		return relationmigrate.Receipt{}, err
	}
	receipt := relationmigrate.Receipt{
		ScopeKey: session.ScopeKey, SourcePairs: preview.SourcePairs, Dangling: preview.Dangling, Conflicts: preview.Conflicts,
	}
	if preview.Blocked {
		receipt.Status = "failed"
		receipt.Error = strings.Join(preview.Blockers, "; ")
		return receipt, fmt.Errorf("%s", receipt.Error)
	}
	if preview.TransitionKind == relationmigrate.KindNoOp {
		receipt.Status = "passed"
		receipt.DestinationPairs = preview.ValidPairs
		receipt.Idempotent = true
		return receipt, nil
	}
	src, err := session.inspect(req.FromModel, req.FromRelation, req.ToModel, req.ToRelation, req.KnownAs)
	if err != nil {
		return receipt, err
	}
	saved := uniquePairs(src.pairs)
	if !src.present {
		have, herr := session.inspect(req.FromModel, req.TargetFromRelation, req.ToModel, req.TargetToRelation, req.KnownAs)
		if herr != nil {
			return receipt, herr
		}
		saved = uniquePairs(have.pairs)
		receipt.Idempotent = true
	}
	dst, err := describeStorage(req.FromModel, req.TargetFromRelation, req.ToModel, req.TargetToRelation, req.KnownAs)
	if err != nil {
		return receipt, err
	}
	if err := session.ensureStorage(dst); err != nil {
		return receipt, err
	}
	if err := session.copyPairs(dst, req, saved); err != nil {
		return receipt, err
	}
	got, err := session.inspect(req.FromModel, req.TargetFromRelation, req.ToModel, req.TargetToRelation, req.KnownAs)
	if err != nil {
		return receipt, err
	}
	if !pairSetEqual(saved, got.pairs) {
		receipt.Status = "failed"
		receipt.Error = "target pair set does not match the source"
		return receipt, fmt.Errorf("%s", receipt.Error)
	}
	dropped, err := session.dropObsolete(src.spec, dst)
	if err != nil {
		return receipt, err
	}
	final, err := session.inspect(req.FromModel, req.TargetFromRelation, req.ToModel, req.TargetToRelation, req.KnownAs)
	if err != nil {
		return receipt, err
	}
	if !pairSetEqual(saved, final.pairs) {
		receipt.Status = "failed"
		receipt.Error = "pair set changed while dropping obsolete storage"
		return receipt, fmt.Errorf("%s", receipt.Error)
	}
	receipt.Status = "passed"
	receipt.CopiedPairs = len(saved)
	receipt.DestinationPairs = len(final.pairs)
	receipt.Dropped = dropped
	return receipt, nil
}

// VerifyRelationCardinality checks the target shape and pair degree without writing.
func VerifyRelationCardinality(session SQLSession, req relationmigrate.Request) (relationmigrate.Receipt, error) {
	req = req.Normalized()
	got, err := session.inspect(req.FromModel, req.TargetFromRelation, req.ToModel, req.TargetToRelation, req.KnownAs)
	if err != nil {
		return relationmigrate.Receipt{}, err
	}
	conflicts := degreeConflicts(got.pairs, req.FromModel, req.TargetFromRelation, req.ToModel, req.TargetToRelation)
	receipt := relationmigrate.Receipt{
		ScopeKey: session.ScopeKey, Status: "passed", DestinationPairs: len(uniquePairs(got.pairs)), Conflicts: conflicts, Dangling: got.dangling,
	}
	if !got.present {
		receipt.Status = "failed"
		receipt.Error = "target relation storage is missing"
		return receipt, fmt.Errorf("%s", receipt.Error)
	}
	if conflicts > 0 {
		receipt.Status = "failed"
		receipt.Error = "target degree conflict"
		return receipt, fmt.Errorf("%s", receipt.Error)
	}
	return receipt, nil
}

func (s SQLSession) ensureStorage(spec storageSpec) error {
	if spec.Pivot != "" {
		left, right := spec.PivotLeft, spec.PivotRight
		q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			%s VARCHAR(36) REFERENCES %s (id) ON DELETE CASCADE,
			%s VARCHAR(36) REFERENCES %s (id) ON DELETE CASCADE,
			PRIMARY KEY (%s, %s)
		)`, s.quote(spec.Pivot), s.quote(left+"_id"), s.quote(left), s.quote(right+"_id"), s.quote(right), s.quote(left+"_id"), s.quote(right+"_id"))
		if err := s.Exec(q); err != nil {
			return err
		}
		for _, col := range []string{left + "_id", right + "_id"} {
			idx := fmt.Sprintf("idx_%s_%s", spec.Pivot, col)
			if err := s.Exec(fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (%s)`, s.quote(idx), s.quote(spec.Pivot), s.quote(col))); err != nil && !ignorableDDL(err) {
				return err
			}
		}
		return nil
	}
	for _, fk := range spec.FKs {
		exists, err := s.columnExists(fk.Holder, fk.Column)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		q := fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s VARCHAR(36) REFERENCES %s (id) ON DELETE CASCADE`,
			s.quote(fk.Holder), s.quote(fk.Column), s.quote(fk.Ref))
		if err := s.Exec(q); err != nil && !ignorableDDL(err) {
			return err
		}
	}
	return nil
}

func (s SQLSession) copyPairs(spec storageSpec, req relationmigrate.Request, pairs []relationmigrate.Pair) error {
	if spec.Pivot != "" {
		for _, p := range pairs {
			q := insertIgnore(s.Engine, s.quote(spec.Pivot), s.quote(spec.PivotLeft+"_id"), s.quote(spec.PivotRight+"_id"))
			if err := s.ExecArgs(q, p.Left, p.Right); err != nil {
				return err
			}
		}
		return nil
	}
	left, _ := sortedPhysicalPair(req.FromModel, req.ToModel)
	for _, fk := range spec.FKs {
		for _, p := range pairs {
			holderID, refID := p.Left, p.Right
			if fk.Holder != left {
				holderID, refID = p.Right, p.Left
			}
			q := fmt.Sprintf(`UPDATE %s SET %s = ? WHERE %s = ? AND (%s IS NULL OR %s = '' OR %s = ?)`,
				s.quote(fk.Holder), s.quote(fk.Column), s.quote("id"), s.quote(fk.Column), s.quote(fk.Column), s.quote(fk.Column))
			if err := s.ExecArgs(q, refID, holderID, refID); err != nil {
				return err
			}
		}
		if fk.Unique {
			idx := fmt.Sprintf("idx_%s_%s_unique", fk.Holder, fk.Column)
			if err := s.Exec(fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (%s)`, s.quote(idx), s.quote(fk.Holder), s.quote(fk.Column))); err != nil && !ignorableDDL(err) {
				return err
			}
		} else {
			idx := fmt.Sprintf("idx_%s_%s_unique", fk.Holder, fk.Column)
			_ = s.Exec(fmt.Sprintf(`DROP INDEX IF EXISTS %s`, s.quote(idx)))
		}
	}
	return nil
}

func (s SQLSession) dropObsolete(src, dst storageSpec) ([]string, error) {
	var dropped []string
	keep := map[string]struct{}{}
	for _, fk := range dst.FKs {
		keep[fk.Holder+"."+fk.Column] = struct{}{}
	}
	for _, fk := range src.FKs {
		key := fk.Holder + "." + fk.Column
		if _, ok := keep[key]; ok {
			if !fk.Unique || !dstFKUnique(dst, fk) {
				idx := fmt.Sprintf("idx_%s_%s_unique", fk.Holder, fk.Column)
				_ = s.Exec(fmt.Sprintf(`DROP INDEX IF EXISTS %s`, s.quote(idx)))
			}
			continue
		}
		ok, err := s.columnExists(fk.Holder, fk.Column)
		if err != nil {
			return dropped, err
		}
		if !ok {
			continue
		}
		if s.DropColumn == nil {
			return dropped, fmt.Errorf("drop column is not available")
		}
		if err := s.DropColumn(fk.Holder, fk.Column); err != nil {
			return dropped, err
		}
		dropped = append(dropped, key)
	}
	if src.Pivot != "" && src.Pivot != dst.Pivot {
		ok, err := s.tableExists(src.Pivot)
		if err != nil {
			return dropped, err
		}
		if ok {
			if err := s.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS %s`, s.quote(src.Pivot))); err != nil {
				return dropped, err
			}
			dropped = append(dropped, src.Pivot)
		}
	}
	return dropped, nil
}

func dstFKUnique(dst storageSpec, fk fkSpec) bool {
	for _, d := range dst.FKs {
		if d.Holder == fk.Holder && d.Column == fk.Column {
			return d.Unique
		}
	}
	return false
}

func insertIgnore(engine, table, c1, c2 string) string {
	if isPostgresEngine(engine) {
		return fmt.Sprintf(`INSERT INTO %s (%s, %s) VALUES (?, ?) ON CONFLICT DO NOTHING`, table, c1, c2)
	}
	if sqlcommon.EngineIsSQLiteLike(engine) {
		return fmt.Sprintf(`INSERT OR IGNORE INTO %s (%s, %s) VALUES (?, ?)`, table, c1, c2)
	}
	return fmt.Sprintf(`INSERT IGNORE INTO %s (%s, %s) VALUES (?, ?)`, table, c1, c2)
}

// DropSQLColumn drops a foreign-key column on PostgreSQL, MySQL, or MariaDB.
// SQLite-like engines use the driver's rebuild helper instead.
func DropSQLColumn(ctx context.Context, db *bun.DB, engine, table, column string) error {
	if db == nil {
		return fmt.Errorf("nil database")
	}
	q := func(ident string) string {
		if isPostgresEngine(engine) {
			return sqlcommon.QuotePGIdent(ident)
		}
		return sqlcommon.QuoteMySQLIdent(ident)
	}
	if isPostgresEngine(engine) {
		_, err := db.NewRaw(fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s CASCADE", q(table), q(column))).Exec(ctx)
		return err
	}
	rows, err := db.QueryContext(ctx, fmt.Sprintf(
		`SELECT CONSTRAINT_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = '%s' AND COLUMN_NAME = '%s' AND REFERENCED_TABLE_NAME IS NOT NULL`,
		strings.ReplaceAll(table, "'", ""), strings.ReplaceAll(column, "'", "")))
	if err == nil && rows != nil {
		for rows.Next() {
			var name string
			if scanErr := rows.Scan(&name); scanErr == nil && name != "" {
				_, _ = db.NewRaw(fmt.Sprintf("ALTER TABLE %s DROP FOREIGN KEY %s", q(table), q(name))).Exec(ctx)
			}
		}
		_ = rows.Close()
	}
	_, err = db.NewRaw(fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", q(table), q(column))).Exec(ctx)
	return err
}

func ignorableDDL(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column") || strings.Contains(msg, "already exists")
}

// NewBunSession binds a Bun database to SQLSession. dropColumn handles SQLite rebuilds.
func NewBunSession(ctx context.Context, engine, scope string, db *bun.DB, dropColumn func(table, column string) error) SQLSession {
	return SQLSession{
		Engine: engine, ScopeKey: scope,
		Exec: func(query string) error {
			_, err := db.NewRaw(query).Exec(ctx)
			return err
		},
		ExecArgs: func(query string, args ...interface{}) error {
			_, err := db.NewRaw(query, args...).Exec(ctx)
			return err
		},
		Query: func(query string) ([][]string, error) {
			rows, err := db.QueryContext(ctx, query)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			cols, err := rows.Columns()
			if err != nil {
				return nil, err
			}
			var out [][]string
			for rows.Next() {
				raw := make([]interface{}, len(cols))
				ptrs := make([]interface{}, len(cols))
				for i := range raw {
					ptrs[i] = &raw[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return nil, err
				}
				line := make([]string, len(cols))
				for i, v := range raw {
					switch t := v.(type) {
					case nil:
						line[i] = ""
					case string:
						line[i] = t
					case []byte:
						line[i] = string(t)
					default:
						line[i] = fmt.Sprint(t)
					}
				}
				out = append(out, line)
			}
			return out, rows.Err()
		},
		DropColumn: dropColumn,
	}
}

