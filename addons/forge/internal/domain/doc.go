// Copyright 2026 dezeat
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

// Package domain holds Forge's pure value types and the logic that belongs to
// them. It mirrors the core's domain layer and obeys the same law: no adapter
// imports, no I/O, no network.
//
// It differs from the core's domain in exactly one permitted way. Forge types
// reference core domain types — a draft holds a core pack, a candidate becomes
// a core pack question — because D-015 fixes the dependency direction as
// Forge -> core and forbids the reverse. That is the one-way import, not a
// layering breach: the core neither imports nor knows this package exists.
package domain
