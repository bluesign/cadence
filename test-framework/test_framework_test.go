/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2022 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/tests/checker"
)

func TestRunningMultipleTests(t *testing.T) {
	t.Parallel()

	code := `
        pub fun testFunc1() {
            assert(false)
        }

        pub fun testFunc2() {
            assert(true)
        }
    `

	runner := NewTestRunner()
	results, err := runner.RunTests(code)
	require.NoError(t, err)

	require.Len(t, results, 2)
	assert.Error(t, results["testFunc1"])
	assert.NoError(t, results["testFunc2"])
}

func TestRunningSingleTest(t *testing.T) {
	t.Parallel()

	code := `
        pub fun testFunc1() {
            assert(false)
        }

        pub fun testFunc2() {
            assert(true)
        }
    `

	runner := NewTestRunner()

	result, err := runner.RunTest(code, "testFunc1")
	assert.NoError(t, err)
	assert.Error(t, result)

	result, err = runner.RunTest(code, "testFunc2")
	assert.NoError(t, err)
	assert.NoError(t, result)
}

func TestExecuteScript(t *testing.T) {
	t.Parallel()

	code := `
        import Test

        pub fun test() {
            var blockchain = Test.newEmulatorBlockchain()
            var result = blockchain.executeScript("pub fun main(): Int {  return 2 + 3 }")

            assert(result.status == Test.ResultStatus.succeeded)
            assert((result.returnValue! as! Int) == 5)

            log(result.returnValue)
        }
    `
	runner := NewTestRunner()
	result, err := runner.RunTest(code, "test")
	assert.NoError(t, err)
	assert.NoError(t, result)
}

func TestImportContract(t *testing.T) {
	t.Parallel()

	t.Run("init no params", func(t *testing.T) {
		t.Parallel()

		code := `
            import FooContract from "./FooContract"

            pub fun test() {
                var foo = FooContract()
                var result = foo.sayHello()
                assert(result == "hello from Foo")
            }
        `

		fooContract := `
            pub contract FooContract {
                init() {}

                pub fun sayHello(): String {
                    return "hello from Foo"
                }
            }
        `

		importResolver := func(location common.Location) (string, error) {
			return fooContract, nil
		}

		runner := NewTestRunner().WithImportResolver(importResolver)

		result, err := runner.RunTest(code, "test")
		assert.NoError(t, err)
		assert.NoError(t, result)
	})

	t.Run("init with params", func(t *testing.T) {
		t.Parallel()

		code := `
            import FooContract from "./FooContract"

            pub fun test() {
                var foo = FooContract(greeting: "hello from Foo")
                var result = foo.sayHello()
                assert(result == "hello from Foo")
            }
        `

		fooContract := `
            pub contract FooContract {

                pub var greeting: String

                init(greeting: String) {
                    self.greeting = greeting
                }

                pub fun sayHello(): String {
                    return self.greeting
                }
            }
        `

		importResolver := func(location common.Location) (string, error) {
			return fooContract, nil
		}

		runner := NewTestRunner().WithImportResolver(importResolver)

		result, err := runner.RunTest(code, "test")
		assert.NoError(t, err)
		assert.NoError(t, result)
	})

	t.Run("invalid import", func(t *testing.T) {
		t.Parallel()

		code := `
            import FooContract from "./FooContract"

            pub fun test() {
                var foo = FooContract()
            }
        `

		importResolver := func(location common.Location) (string, error) {
			return "", errors.New("cannot load file")
		}

		runner := NewTestRunner().WithImportResolver(importResolver)

		_, err := runner.RunTest(code, "test")
		require.Error(t, err)

		errs := checker.ExpectCheckerErrors(t, err, 2)

		importedProgramError := &sema.ImportedProgramError{}
		assert.ErrorAs(t, errs[0], &importedProgramError)
		assert.Contains(t, importedProgramError.Err.Error(), "cannot load file")

		assert.IsType(t, &sema.NotDeclaredError{}, errs[1])
	})

	t.Run("import resolver not provided", func(t *testing.T) {
		t.Parallel()

		code := `
            import FooContract from "./FooContract"

            pub fun test() {
                var foo = FooContract()
            }
        `

		runner := NewTestRunner()
		_, err := runner.RunTest(code, "test")
		require.Error(t, err)

		errs := checker.ExpectCheckerErrors(t, err, 2)

		importedProgramError := &sema.ImportedProgramError{}
		require.ErrorAs(t, errs[0], &importedProgramError)
		assert.IsType(t, ImportResolverNotProvidedError{}, importedProgramError.Err)

		assert.IsType(t, &sema.NotDeclaredError{}, errs[1])
	})

	t.Run("nested imports", func(t *testing.T) {
		t.Parallel()

		code := `
            import FooContract from "./FooContract"

            pub fun test() {}
        `

		fooContract := `
           import BarContract from 0x01

            pub contract FooContract {
                init() {}
            }
        `
		barContract := `
            pub contract BarContract {
                init() {}
            }
        `

		importResolver := func(location common.Location) (string, error) {
			switch location := location.(type) {
			case common.StringLocation:
				if location == "./FooContract" {
					return fooContract, nil
				}
			case common.AddressLocation:
				if location.ID() == "A.0000000000000001.BarContract" {
					return barContract, nil
				}
			}

			return "", fmt.Errorf("unsupported import %s", location.ID())
		}

		runner := NewTestRunner().WithImportResolver(importResolver)

		_, err := runner.RunTest(code, "test")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nested imports are not supported")
	})
}

func TestUsingEnv(t *testing.T) {
	t.Parallel()

	t.Run("public key creation", func(t *testing.T) {
		t.Parallel()

		code := `
            pub fun test() {
                var publicKey = PublicKey(
                    publicKey: "1234".decodeHex(),
                    signatureAlgorithm: SignatureAlgorithm.ECDSA_secp256k1
                )
            }
        `

		runner := NewTestRunner()

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)

		require.Error(t, result)
		publicKeyError := interpreter.InvalidPublicKeyError{}
		assert.ErrorAs(t, result, &publicKeyError)
	})

	t.Run("public account", func(t *testing.T) {
		t.Parallel()

		code := `
            pub fun test() {
                var acc = getAccount(0x01)
                var bal = acc.balance
                assert(acc.balance == 0.0)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		assert.NoError(t, err)
		assert.NoError(t, result)
	})

	t.Run("auth account", func(t *testing.T) {
		t.Parallel()

		code := `
            pub fun test() {
                var acc = getAuthAccount(0x01)
                var bal = acc.balance
                assert(acc.balance == 0.0)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		assert.NoError(t, err)
		assert.NoError(t, result)
	})

	// Imported programs also should have the access to the env.
	t.Run("account access in imported program", func(t *testing.T) {
		t.Parallel()

		code := `
            import FooContract from "./FooContract"

            pub fun test() {
                var foo = FooContract()
                var result = foo.getBalance()
                assert(result == 0.0)
            }
        `

		fooContract := `
            pub contract FooContract {
                init() {}

                pub fun getBalance(): UFix64 {
                    var acc = getAccount(0x01)
                    return acc.balance
                }
            }
        `

		importResolver := func(location common.Location) (string, error) {
			return fooContract, nil
		}

		runner := NewTestRunner().WithImportResolver(importResolver)

		result, err := runner.RunTest(code, "test")
		assert.NoError(t, err)
		assert.NoError(t, result)
	})
}

func TestCreateAccount(t *testing.T) {
	t.Parallel()

	code := `
        import Test

        pub fun test() {
            var blockchain = Test.newEmulatorBlockchain()
            var account = blockchain.createAccount()
        }
    `

	runner := NewTestRunner()
	result, err := runner.RunTest(code, "test")
	assert.NoError(t, err)
	assert.NoError(t, result)
}

func TestExecutingTransactions(t *testing.T) {
	t.Parallel()

	t.Run("add transaction", func(t *testing.T) {
		t.Parallel()

		code := `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                let tx = Test.Transaction(
                    "transaction { execute{ assert(false) } }",
                    account.address,
                    [account]
                )

                blockchain.addTransaction(tx)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		assert.NoError(t, err)
		assert.NoError(t, result)
	})

	t.Run("run next transaction", func(t *testing.T) {
		t.Parallel()

		code := `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                let tx = Test.Transaction(
                    "transaction { execute{ assert(true) } }",
                    nil,
                    [account]
                )

                blockchain.addTransaction(tx)

                let result = blockchain.executeNextTransaction()!
                assert(result.status == Test.ResultStatus.succeeded)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		assert.NoError(t, err)
		assert.NoError(t, result)
	})

	t.Run("run next transaction with authorizer", func(t *testing.T) {
		t.Parallel()

		code := `
            import Test

            pub fun test() {
                let blockchain = Test.newEmulatorBlockchain()
                let account = blockchain.createAccount()

                let tx = Test.Transaction(
                    "transaction { prepare(acct: AuthAccount) {} execute{ assert(true) } }",
                    account.address,
                    [account]
                )

                blockchain.addTransaction(tx)

                let result = blockchain.executeNextTransaction()!
                assert(result.status == Test.ResultStatus.succeeded)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		assert.NoError(t, err)
		assert.NoError(t, result)
	})

	t.Run("transaction failure", func(t *testing.T) {
		t.Parallel()

		code := `
            import Test

            pub fun test() {
                let blockchain = Test.newEmulatorBlockchain()
                let account = blockchain.createAccount()

                let tx = Test.Transaction(
                    "transaction { execute{ assert(false) } }",
                    nil,
                    [account]
                )

                blockchain.addTransaction(tx)

                let result = blockchain.executeNextTransaction()!
                assert(result.status == Test.ResultStatus.failed)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		assert.NoError(t, err)
		assert.NoError(t, result)
	})

	t.Run("run non existing transaction", func(t *testing.T) {
		t.Parallel()

		code := `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                let result = blockchain.executeNextTransaction()
                assert(result == nil)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		assert.NoError(t, err)
		assert.NoError(t, result)
	})

	t.Run("commit block", func(t *testing.T) {
		t.Parallel()

		code := `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                blockchain.commitBlock()
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		assert.NoError(t, err)
		assert.NoError(t, result)
	})

	t.Run("commit un-executed block", func(t *testing.T) {
		t.Parallel()

		code := `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                let account = blockchain.createAccount()

                let tx = Test.Transaction(
                    "transaction { execute{ assert(false) } }",
                    nil,
                    [account]
                )

                blockchain.addTransaction(tx)

                blockchain.commitBlock()
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)

		require.Error(t, result)
		assert.Contains(t, result.Error(), "cannot be committed before execution")
	})

	t.Run("commit partially executed block", func(t *testing.T) {
		t.Parallel()

		code := `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                let account = blockchain.createAccount()

                let tx = Test.Transaction(
                    "transaction { execute{ assert(false) } }",
                    nil,
                    [account]
                )

                // Add two transactions
                blockchain.addTransaction(tx)
                blockchain.addTransaction(tx)

                // But execute only one
                blockchain.executeNextTransaction()

                // Then try to commit
                blockchain.commitBlock()
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)

		require.Error(t, result)
		assert.Contains(t, result.Error(), "is currently being executed")
	})

	t.Run("multiple commit block", func(t *testing.T) {
		t.Parallel()

		code := `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                blockchain.commitBlock()
                blockchain.commitBlock()
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		assert.NoError(t, err)
		assert.NoError(t, result)
	})

	t.Run("run given transaction", func(t *testing.T) {
		t.Parallel()

		code := `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                let tx = Test.Transaction(
                    "transaction { execute{ assert(true) } }",
                    nil,
                    [account]
                )

                let result = blockchain.executeTransaction(tx)!
                assert(result.status == Test.ResultStatus.succeeded)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		assert.NoError(t, err)
		assert.NoError(t, result)
	})

	t.Run("run given transaction unsuccessful", func(t *testing.T) {
		t.Parallel()

		code := `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                let tx = Test.Transaction(
                    "transaction { execute{ assert(fail) } }",
                    nil,
                    [account]
                )

                let result = blockchain.executeTransaction(tx)!
                assert(result.status == Test.ResultStatus.failed)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		assert.NoError(t, err)
		assert.NoError(t, result)
	})

	t.Run("run multiple transactions", func(t *testing.T) {
		t.Parallel()

		code := `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                let tx1 = Test.Transaction(
                    "transaction { execute{ assert(true) } }",
                    nil,
                    [account]
                )

                let tx2 = Test.Transaction(
                    "transaction { prepare(acct: AuthAccount) {} execute{ assert(true) } }",
                    account.address,
                    [account]
                )

                let tx3 = Test.Transaction(
                    "transaction { execute{ assert(false) } }",
                    nil,
                    [account]
                )

                let firstResults = blockchain.executeTransactions([tx1, tx2, tx3])!

                assert(firstResults.length == 3)
                assert(firstResults[0].status == Test.ResultStatus.succeeded)
                assert(firstResults[1].status == Test.ResultStatus.succeeded)
                assert(firstResults[2].status == Test.ResultStatus.failed)


                // Execute them again: To verify the proper increment/reset of sequence numbers.
                let secondResults = blockchain.executeTransactions([tx1, tx2, tx3])!

                assert(secondResults.length == 3)
                assert(secondResults[0].status == Test.ResultStatus.succeeded)
                assert(secondResults[1].status == Test.ResultStatus.succeeded)
                assert(secondResults[2].status == Test.ResultStatus.failed)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		assert.NoError(t, err)
		assert.NoError(t, result)
	})

	t.Run("run empty transactions", func(t *testing.T) {
		t.Parallel()

		code := `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                let result = blockchain.executeTransactions([])!
                assert(result.length == 0)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		assert.NoError(t, err)
		assert.NoError(t, result)
	})

	t.Run("run transaction with pending transactions", func(t *testing.T) {
		t.Parallel()

		code := `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                let tx1 = Test.Transaction(
                    "transaction { execute{ assert(true) } }",
                    nil,
                    [account]
                )

                blockchain.addTransaction(tx1)

                let tx2 = Test.Transaction(
                    "transaction { execute{ assert(true) } }",
                    nil,
                    [account]
                )
                let result = blockchain.executeTransaction(tx2)!

                assert(result.status == Test.ResultStatus.succeeded)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)

		require.Error(t, result)
		assert.Contains(t, result.Error(), "is currently being executed")
	})
}

func TestSetupAndTearDown(t *testing.T) {
	t.Parallel()

	t.Run("setup", func(t *testing.T) {
		t.Parallel()

		code := `
            pub(set) var setupRan = false

            pub fun setup() {
                assert(!setupRan)
                setupRan = true
            }

            pub fun testFunc() {
                assert(setupRan)
            }
        `

		runner := NewTestRunner()
		results, err := runner.RunTests(code)
		require.NoError(t, err)

		require.Len(t, results, 1)
		assert.NoError(t, results["testFunc"])
	})

	t.Run("setup failed", func(t *testing.T) {
		t.Parallel()

		code := `
            pub fun setup() {
                panic("error occurred")
            }

            pub fun testFunc() {
                assert(true)
            }
        `

		runner := NewTestRunner()
		results, err := runner.RunTests(code)
		require.Error(t, err)
		require.Empty(t, results)
	})

	t.Run("teardown", func(t *testing.T) {
		t.Parallel()

		code := `
            pub(set) var tearDownRan = false

            pub fun testFunc() {
                assert(!tearDownRan)
            }

            pub fun tearDown() {
                assert(true)
            }
        `

		runner := NewTestRunner()
		results, err := runner.RunTests(code)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.NoError(t, results["testFunc"])
	})

	t.Run("teardown failed", func(t *testing.T) {
		t.Parallel()

		code := `
            pub(set) var tearDownRan = false

            pub fun testFunc() {
                assert(!tearDownRan)
            }

            pub fun tearDown() {
                assert(false)
            }
        `

		runner := NewTestRunner()
		results, err := runner.RunTests(code)

		// Running tests will return an error since the tear down failed.
		require.Error(t, err)

		// However, test cases should have been passed.
		require.Len(t, results, 1)
		assert.NoError(t, results["testFunc"])
	})
}
