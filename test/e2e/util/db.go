// Copyright 2018 Oracle and/or its affiliates. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"os/exec"
	"strings"
)

// DBTestHelper can be used to quickly create and check some basic database entities.
type DBTestHelper interface {
	CreateDB(db string)
	HasDB(db string) (result bool)
	DeleteDB(db string)
	CreateDBTable(db string, table string)
	HasDBTable(db string, table string) (result bool)
	CreateDBTableValue(db string, table string, value string)
	HasDBTableValue(db string, table string, value string) (result bool)
	EnsureDBTableValue(db string, table string, value string)
}

// MySQLDBTestHelper can be used to quickly create and check some basic MySQL database entities.
type MySQLDBTestHelper struct {
	t  *T
	ex SimpleSQLExecutor
}

// NewMySQLDBTestHelper creates a MySQLDBTestHelper with the specified SimpleSQLExecutor.
func NewMySQLDBTestHelper(t *T, executor SimpleSQLExecutor) *MySQLDBTestHelper {
	return &MySQLDBTestHelper{
		t:  t,
		ex: executor,
	}
}

// CreateDB creates a database.
func (dbh *MySQLDBTestHelper) CreateDB(db string) {
	_, e := dbh.ex.ExecuteSQL("create database " + db + ";")
	if e != nil {
		dbh.t.Fatalf("Error creating database '%v': %v", db, e)
	}
	dbExists := dbh.HasDB(db)
	if !dbExists {
		dbh.t.Fatalf("Error database '%v' was not created.", db)
	}
}

// HasDB returns true the specified database exists; false otherwise.
func (dbh *MySQLDBTestHelper) HasDB(db string) bool {
	out, e := dbh.ex.ExecuteSQL("show databases;")
	if e != nil {
		dbh.t.Fatalf("Error checking database '%v' existence: %v", db, e)
	} else {
		return hasRowColumnValue(out, db)
	}
	return false
}

// DeleteDB deletes the specified database if it exists.
func (dbh *MySQLDBTestHelper) DeleteDB(db string) {
	dbExists := dbh.HasDB(db)
	if dbExists {
		_, e := dbh.ex.ExecuteSQL("drop database " + db + ";")
		if e != nil {
			dbh.t.Fatalf("Error deleting database '%v: %v", db, e)
		}
		dbExists := dbh.HasDB(db)
		if dbExists {
			dbh.t.Fatalf("Error database '%v' was not deleted.", db)
		}
	}
}

// CreateDBTable creates a table in the specified database.
func (dbh *MySQLDBTestHelper) CreateDBTable(db string, table string, column string) {
	_, e := dbh.ex.ExecuteSQLForDB(db, "create table "+table+" ("+column+" varchar(256) NOT NULL PRIMARY KEY);")
	if e != nil {
		dbh.t.Fatalf("Error checking database table '%v.%v': %v", db, table, e)
	}
	dbTableExists := dbh.HasDBTable(db, table)
	if !dbTableExists {
		dbh.t.Fatalf("Error database table '%v.%v' was not created.", db, table)
	}
}

// HasDBTable returns true the specified database table exists; false otherwise.
func (dbh *MySQLDBTestHelper) HasDBTable(db string, table string) bool {
	out, e := dbh.ex.ExecuteSQLForDB(db, "show tables;")
	if e != nil {
		dbh.t.Fatalf(
			"Error checking database table '%v.%v' existence: %v", db, table, e)
	} else {
		return hasRowColumnValue(out, table)
	}
	return false
}

// CreateDBTableValue creates a value in the specified database table.
func (dbh *MySQLDBTestHelper) CreateDBTableValue(db string, table string, column string, value string) {
	_, e := dbh.ex.ExecuteSQLForDB(db, "insert into "+table+" ("+column+") values(\""+value+"\");")
	if e != nil {
		dbh.t.Fatalf("Error checking database table '%v.%v.%v' for value '%v': %v", db, table, column, value, e)
	}
	dbValueExists := dbh.HasDBTableValue(db, table, column, value)
	if !dbValueExists {
		dbh.t.Fatalf("Error database table '%v.%v.%v' did not contain value '%v'.", db, table, column, value)
	}
}

// HasDBTableValue returns true the specified database table value exists; false otherwise.
func (dbh *MySQLDBTestHelper) HasDBTableValue(db string, table string, column string, value string) bool {
	out, e := dbh.ex.ExecuteSQLForDB(db, "select "+column+" from "+table+";")
	if e != nil {
		dbh.t.Fatalf("Error checking database table '%v.%v.%v' for value '%v': %v", db, table, column, value, e)
	} else {
		return hasRowColumnValue(out, value)
	}
	return false
}

// EnsureDBTableValue create the specified value in the datbase along with the
// required db and table if they do not already exists.
func (dbh *MySQLDBTestHelper) EnsureDBTableValue(db string, table string, column string, value string) {
	if dbh.HasDB(db) {
		dbh.DeleteDB(db)
	}
	dbh.CreateDB(db)
	dbh.CreateDBTable(db, table, column)
	dbh.CreateDBTableValue(db, table, column, value)
}

// SimpleSQLExecutor is a simple interface for executing SQL operations against a database.
// An SQL command string is executed; and the full output of the command is returned.
// Client's can parse the output string as required.
type SimpleSQLExecutor interface {
	ExecuteSQL(sql string) (output string, e error)
	ExecuteSQLForDB(sql string, db string) (output string, e error)
}

// KubectlSimpleSQLExecutor uses kubectl (no dependencies) to implement the SimpleSQLExecutor
// interface.
type KubectlSimpleSQLExecutor struct {
	t         *T
	podname   string
	username  string
	password  string
	namespace string
}

// NewKubectlSimpleSQLExecutor creates a KubectlSimpleSQLExecutor.
func NewKubectlSimpleSQLExecutor(
	t *T,
	podname string,
	username string,
	password string,
	namespace string,
) *KubectlSimpleSQLExecutor {
	return &KubectlSimpleSQLExecutor{
		t:         t,
		podname:   podname,
		username:  username,
		password:  password,
		namespace: namespace,
	}
}

// ExecuteSQL executes the specified SQL command using kubectl via exec.
func (kse KubectlSimpleSQLExecutor) ExecuteSQL(sql string) (string, error) {
	cmd := exec.Command(
		"kubectl",
		"-n",
		kse.namespace,
		"exec",
		kse.podname,
		"-c",
		"mysql",
		"--",
		"bash",
		"-c",
		"\"\"/bin/mysql -u"+kse.username+" -p"+kse.password+" -e '"+sql+"'\"\"",
	)

	output, err := executeCmd(kse.t, cmd)
	if err != nil {
		kse.t.Errorf("Error executing command: %v %v", cmd, err)
	}
	return output, err
}

// ExecuteCMD executes the specified command using kubectl via exec.
func (kse KubectlSimpleSQLExecutor) ExecuteCMD(cmdStr string) (string, error) {
	cmd := exec.Command(
		"kubectl",
		"-n",
		kse.namespace,
		"exec",
		kse.podname,
		"-c",
		"mysql",
		"--",
		"bash",
		"-c",
		cmdStr,
	)

	output, err := executeCmd(kse.t, cmd)
	return output, err
}

// ExecuteSQLForDB executes the specified SQL command against the specified db
// using kubectl via exec.
func (kse KubectlSimpleSQLExecutor) ExecuteSQLForDB(db string, sql string) (string, error) {
	cmd := exec.Command(
		"kubectl",
		"-n",
		kse.namespace,
		"exec",
		kse.podname,
		"-c",
		"mysql",
		"--",
		"bash",
		"-c",
		"\"\"/bin/mysql"+" -u"+kse.username+" -p"+kse.password+" -D"+db+" -e '"+sql+"'\"\"",
	)

	output, err := executeCmd(kse.t, cmd)
	if err != nil {
		kse.t.Errorf("Error executing command: %v %v", cmd, err)
	}
	return output, err
}

// hasRowColumnValue is a helper function that searches exactly (exclusively
// and completely on one row) for the specified string in a '\n' delimitted
// string.
func hasRowColumnValue(out string, value string) bool {
	for _, row := range strings.Split(out, "\n") {
		if row == value {
			return true
		}
	}
	return false
}

// executeCmd executes the specified command and returns all output.
func executeCmd(t *T, cmd *exec.Cmd) (string, error) {
	output, e := cmd.CombinedOutput()
	if e != nil {
		t.Logf("Failed to execute command:%v: %v", cmd.Args, e)
	}
	return string(output), e
}

// GetMySQLPassword is a helper method to get the MYSQL_ROOT_PASSWORD from a running pod.
func GetMySQLPassword(t *T, podname string, namespace string) string {
	cmd := exec.Command(
		"kubectl",
		"-n",
		namespace,
		"exec",
		podname,
		"-c",
		"mysql",
		"--",
		"bash",
		"-c",
		"env | grep MYSQL_ROOT_PASSWORD",
	)
	output, err := executeCmd(t, cmd)
	if err != nil {
		t.Errorf("Error executing command: %v %v", cmd, err)
	}
	return strings.TrimSpace(strings.SplitN(output, "=", 2)[1])
}
