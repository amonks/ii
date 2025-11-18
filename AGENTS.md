These are notes for a table-top roleplaying campaign, with a cross-referencing system to make them easy to browse.

- `notes/sessions/*.md` are the source files, and we create `notes/summaries (ai-generated)/**/*.md` from them. We call the former "session files" or "session notes", and the latter "tag files" or "summary files".
- I wrote the session notes, and they are the source of truth.
- for searchability, we have a system where characters, locations, themes, objects, etc get ALLCAPS TAGS

# Import procedure

I take raw notes during a session. I'm typing while talking, jotting things down, appending to the end, so the structure can be difficult to follow. "importing" those raw session notes cleans up the formatting and adds cross-references to make the information easier to find. The import procedure has 4 steps.

1. **CLEANUP** We clean up the session notes, adding formatting without changing the text
2. **TAG AUDIT** We perform a tag audit, identifying the new themes, characters, objects, and locations that correspond to existing tags, plus any references to things that ought to be new tags
3. **SUMMARIZATION** We update the summary files: location files log everything that happened at a location, person files list all their dialog and all dialog about them, etc.
4. **COMPREHENSIVE CHECK** We read through the _other_ session files to see if they reference anything that we've reified into a tag, and update accordingly.

## CLEANUP process

- read some existing session files to get a feel for the formatting conventions
- update the new session file, breaking it into sections and generally bringing it in line with the expected formatting
- IMPORTANT: YOU MUST NOT CHANGE ANY PROSE IN THE SESSION FILE. YOU MAY ONLY INSERT FORMATTING CHARACTERS. SUBSTITUING ANY WORD IS AN IMMEDIATE FAILURE. The limited exceptions:
  - correcting spelling is ok
  - inserting headers is ok (this counts as formatting)
  - capitalization changes are OK
  - whitespace changes are always ok
  - inserting, removing, or substituting words is never ok

## TAG AUDIT process

- run `tree ./notes` to find the existing tags
- read the new session notes and make a mental list of the characters, locations, themes, and objects referenced. This includes,
  - anything that maatches an existing tag file
  - new things that should have tag files -- err on the side of tagging every character and location, we never know what might turn out to be important. I will sometimes ALLCAPS things when I'm writing the session notes, but that's meaningless; that's just to catch my eye during the session. A chararcter or location should always be a tag, even if it isn't ALLCAPS in the raw session file.
  - objects and themes are tougher: introducing a tag for every random object would be absurd. Use your judgement to propose a list of objects and themes we might want to add tags for
- modify the session file to use tags consistently
  - many characters have multiple names (eg codenames, first and last, nicknames). Any line where any of these names appears must be tagged.

## SUMMARIZATION process

- "summarization" is a misnomer: we're really just extracting excerpts
- tag files should never contain analysis, just quotes and facts
- read a handful of each type of summary file to get a feel for the formatting and content conventions
- a tag file should contain surrounding context from the session notes that relate to the tagged object
  - the whole stanza -- in a three-way conversation, we would include all three characters dialog in all three of their tag files
- for each tag that comes up in the session file, add the appropriate content to that tag's summary file

## COMPREHENSIVE CHECK process

- read through all of the _other_ session files to see if they reference anything that we've reified into a tag
- update accordingly:
  - make sure the tag appears appropriately in the session file
  - make sure that excerpt of the session file appears in the tag file

# GLOBAL INVARIANTS

- every character and location MUST ALWAYS be referred to by their tag. The tag doesn't need to appear on every line, but should always be present.
- each tag MUST have a corresponding tag file.
- summary files should ALWAYS quote directly from session notes as much as possible, including surrounding context
- summary files should never conatin analysis, just quotes and facts directly from the session files
