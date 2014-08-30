platypus-preparer
=================

An agent intended to run on each node that runs code. It looks at an intent
store (in this iteration, Consul KV) to figure out which application artifacts
should be present, and fetches and untars them.

See tests for usage.

TODO
----

* Support `runas`, ensure user is present.
* Code cleanup, review.
* Make this README not suck.
* Watch intent store for changes, rather than using a polling loop.
* Enable configuration of artifact repo.
* Support HTTP(S) artifact repos.
* Tests for bogus data in intent store.
* Benchmark extracting tar and decided whether should just be shelling out
  instead.
* Do a scrub over error handling to check for dodgy messages.
