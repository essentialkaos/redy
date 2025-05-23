## Changelog

### 4.4.1

_Bump version with no chagnes_

### 4.3.4

* Added method `GetB` to `Info` struct 
* Added method `Is` to `Info` struct for comparing info fields values

### 4.3.3

* Dependencies update

### 4.3.2

* Removed `pkg.re` usage
* Added module info
* Added Dependabot configuration

### 4.3.1

* Fixed incorrect conversion between integer types

### 4.3.0

* Added method `Has` to `Config` struct for checking config property presence

### 4.2.0

* Return 0 instead of -1 from `Info.GetI` and `Info.GetF` methods if properties are empty

### 4.1.0

* Added method for parsing replica information
* Improved `Info` `Get*` methods for working with renamed properties (`slave` → `replica`)

### 4.0.0

* The total number of keys and expires now calculated on demand
* Added keyspace info to flattened info data
* Using integers for DBs instead of strings

----

### 3.1.0

* Added method `Info.Flatten()` for flattening `Info` data

### 3.0.0

* `InfoSection.Props` was renamed to `InfoSection.Fields`

----

### 2.0.1

* Added fuzz testing
* Fixed bug with parsing keyspace info
* Fixed bug with parsing response data
* Fixed memory leak in bulk and array parsers

### 2.0.0

* `SimpleStr` constant now is `STR_SIMPLE`
* `BulkStr` constant now is `STR_BULK`
* `Int` constant now is `INT`
* `Array` constant now is `ARRAY`
* `Nil` constant now is `NIL`
* `RedisErr` constant now is `ERR_REDIS`
* `IOErr` constant now is `ERR_IO`
* `Str` constant now is `STR`
* `Err` constant now is `ERR`
* `Resp.IsType()` now is `Resp.HasType()`

----

### 1.3.0

* Added method `ParseConfig` for manual configuration parsing

### 1.2.0

Added in-memory configuration parser
Added method `Diff` for configs comparison
Return error if `Cmd`/`PipeResp` executed without connection to Redis instance
Improved configuration file parser
Increased code coverage (66.8% → 96.7%)
Removed dead code

### 1.1.0

* `INFO` output data parser
* Redis configuration file parser

### 1.0.1

* Minor improvements

### 1.0.0

_First public release_
