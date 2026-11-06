// Package updatedispatcher applies a producer's release to a consumer through
// the consumer's update methods. It is the domain behind gero ship and any other
// command that reads or changes what a consumer holds.
//
// Three stages, each with its own input:
//
//   - A UpdateStrategy is reusable Go code that claims a name ("yamlBump") and is
//     listed in a StrategyRegistry, so config can refer to it.
//   - A method in a consumer's config configures a strategy once, with its `with`
//     values, which the strategy validates at load: that gives an Updater.
//   - A producer's downstream edge invokes a method with params. They are checked
//     against the method's allowedParams, which are declared in config and
//     nowhere else, and the Updater reads and writes with the result.
//
// The Updater has a read side (Check, which never writes, for the state check and
// doctor) and a write side (Apply). The Engine builds every method of the loaded
// config, which is also where a strategy's and a method's mistakes are found.
// An Invocation runs the lifecycle around the edit: the method's and the edge's
// pre-edit steps, Apply, then their post-edit steps, with the commands run by a
// StepRunner the caller supplies, so this package starts no process.
package updatedispatcher
