pp-preparer
===========

An agent intended to run on each node that runs code. It looks at an intent
store to figure out which application artifacts should be present, and fetches
and untars them.

See tests for usage.

Status
------

Can determine desired status from intent store, and if required version is not
present fetch artifact from fixed file:// location and untar it.

TODO
----

* Support `runas`, ensure user is present.
* Code cleanup, review.
* Make this README not suck.
* Watch intent store for changes, rather than using a polling loop.
* Enable configuration of artifact repo.
* Support HTTP(S) artifact repos.
* Benchmark extracting tar and decided whether should just be shelling out
  instead.
* Decide whether I even like this go thing.
