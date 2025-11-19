These are notes for a table-top roleplaying campaign, with a cross-referencing system to make them easy to browse.

- `notes/sessions/*.md` are the source files, and we create `notes/summaries (ai-generated)/**/*.md` from them. We call the former "session files" or "session notes", and the latter "tag files" or "tag files".
- I wrote the session notes, and they are the source of truth.
- for searchability, we have a system where characters, locations, themes, objects, etc get ALLCAPS TAGS. TAGS can only contain capital letters and spaces. Each appearance of a tag is automatically hyperlinked to the corresponding tagfile.

# Import procedure

I take raw notes during a session. I'm typing while talking, jotting things down, appending to the end, so the structure can be difficult to follow. "importing" those raw session notes cleans up the formatting and adds cross-references to make the information easier to find. The import procedure has 4 steps.

1. **CLEANUP** We clean up the session notes, adding formatting without changing the text
2. **TAG AUDIT** We perform a tag audit, identifying the new themes, characters, objects, and locations that correspond to existing tags, plus any references to things that ought to be new tags
3. **SUMMARIZATION** We update the tag files: location files log everything that happened at a location, person files list all their dialogue and all dialogue about them, etc.
4. **COMPREHENSIVE CHECK** We read through the _other_ session files to see if they reference anything that we've reified into a tag, and update accordingly.
5. **TAG VERIFICATION** When the previous steps are done for a session, add it to `./imported-sessions` and run `go test`. The test suite enforces:
   - every ALLCAPS string in tagfiles and imported sessions resolves to a tag or approved acronym
   - no tag name exists in more than one category folder
   - every tag that appears in a tagfile also appears somewhere in the session notes

## CLEANUP process

- read some existing session files to get a feel for the formatting conventions
- update the new session file, breaking it into sections and generally bringing it in line with the expected formatting
- when transcribing dialogue, prefer blockquotes with speaker tags so the raw quotes can be copied directly into tag files, e.g.

> LOTTIE: "We're on the record now, so keep it professional."
> SEAMUS: "Copy that. Where do we start?"
> LOTTIE: "Now I'm saying something."

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
- read a handful of each type of tag file to get a feel for the formatting and content conventions
- for each tag that comes up in the session file, add the appropriate content to that tag's tag file
- when copying content to a tag file, always include surrounding context; it is often appropriate to include the whole scene/vignette/section where the tag comes up. For example, dialogue extracts should include the whole conversation, even when parts of that conversation don't relate to the tag.

## COMPREHENSIVE CHECK process

- read through all of the _other_ session files to see if they reference anything that we've reified into a tag
- update accordingly:
  - make sure the tag appears appropriately in the session file
  - make sure that excerpt of the session file appears in the tag file

## TAG VERIFICATION process

- Add the session to `./imported-sessions` and run `go test` to ensure:
  - all ALLCAPS strings in tagfiles and the imported sessions resolve to real tags (or ignored acronyms)
  - every tag file only lives in a single tag category
  - every tag in a tagfile actually appears somewhere in the source session notes

# GLOBAL INVARIANTS

- every character and location MUST ALWAYS be referred to by their tag. The tag doesn't need to appear on every line, but should always be present.
- each tag MUST have a corresponding tag file, and that tag name must also appear in the session notes.
- tag files should ALWAYS quote directly from session notes as much as possible, including surrounding context
- tag files should never conatin analysis, just quotes and facts directly from the session files

# TAG POLICY

Tagging correctly is something of an art.

Sometimes the appropriate tag name won't reasonably appear in the prose, so we can add it in parentheses somewhere, eg,

- three puncture wounds (THREES)
- a german shepherd appears (ANIMALS)
- A piece of paper on the counter says "Charlotte Watkins" (LOTTIE) in black ink.

There are a lot of locations and they can be hard to keep track of -- location tagnames should include city and/or state (as relevant), eg `DENNY'S ZION MD` (punctuation breaks tags, so use spaces to keep multi-word names together).

The correct resolution might be different for different locations -- eg MIAMI might be a good tag, and so might STATE RECORDS FACILITY ZION MD.

- BEDROOM is not a good tag because it is too vague -- which bedroom? in what building?
- BEDROOM CORNUCOPIA HOUSE COTTAGE is not a good tag because it is too specific -- CORNUCOPIA HOUSE is fine
