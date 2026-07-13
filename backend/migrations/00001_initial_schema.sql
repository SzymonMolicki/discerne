-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE language_families (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL
);

CREATE TABLE language_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    family_id UUID NOT NULL REFERENCES language_families (id) ON DELETE RESTRICT,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL
);

CREATE INDEX language_groups_family_id_idx ON language_groups (family_id);

CREATE TABLE language_subgroups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id UUID NOT NULL REFERENCES language_groups (id) ON DELETE RESTRICT,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL
);

CREATE INDEX language_subgroups_group_id_idx ON language_subgroups (group_id);

CREATE TABLE continents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL
);

CREATE TABLE scripts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    iso_15924_code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL
);

CREATE TABLE languages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    iso_639_3 TEXT NOT NULL UNIQUE,
    iso_639_1 TEXT UNIQUE,
    canonical_name TEXT NOT NULL,
    family_id UUID NOT NULL REFERENCES language_families (id) ON DELETE RESTRICT,
    group_id UUID NOT NULL REFERENCES language_groups (id) ON DELETE RESTRICT,
    subgroup_id UUID NOT NULL REFERENCES language_subgroups (id) ON DELETE RESTRICT,
    continent_id UUID NOT NULL REFERENCES continents (id) ON DELETE RESTRICT,
    script_id UUID NOT NULL REFERENCES scripts (id) ON DELETE RESTRICT,
    enabled BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT languages_iso_639_3_format_chk CHECK (iso_639_3 ~ '^[a-z]{3}$'),
    CONSTRAINT languages_iso_639_1_format_chk CHECK (iso_639_1 IS NULL OR iso_639_1 ~ '^[a-z]{2}$')
);

CREATE INDEX languages_enabled_idx ON languages (enabled);
CREATE INDEX languages_family_id_idx ON languages (family_id);
CREATE INDEX languages_group_id_idx ON languages (group_id);
CREATE INDEX languages_subgroup_id_idx ON languages (subgroup_id);
CREATE INDEX languages_continent_id_idx ON languages (continent_id);
CREATE INDEX languages_script_id_idx ON languages (script_id);

CREATE TABLE language_names (
    language_id UUID NOT NULL REFERENCES languages (id) ON DELETE CASCADE,
    locale TEXT NOT NULL,
    name TEXT NOT NULL,
    PRIMARY KEY (language_id, locale),
    CONSTRAINT language_names_locale_chk CHECK (locale IN ('pl-PL', 'en-US', 'es-ES'))
);

CREATE INDEX language_names_locale_idx ON language_names (locale);

CREATE TABLE language_texts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    language_id UUID NOT NULL REFERENCES languages (id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    sentence_count INTEGER NOT NULL,
    source_type TEXT,
    source_reference TEXT,
    license TEXT NOT NULL,
    approved BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT language_texts_sentence_count_chk CHECK (sentence_count IN (2, 3))
);

CREATE INDEX language_texts_language_id_idx ON language_texts (language_id);
CREATE UNIQUE INDEX language_texts_language_id_content_uidx ON language_texts (language_id, content);
CREATE INDEX language_texts_language_approved_idx ON language_texts (language_id, approved);

CREATE TABLE daily_quizzes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    quiz_date DATE NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);

CREATE INDEX daily_quizzes_quiz_date_idx ON daily_quizzes (quiz_date);

CREATE TABLE daily_quiz_questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    daily_quiz_id UUID NOT NULL REFERENCES daily_quizzes (id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    text_id UUID NOT NULL REFERENCES language_texts (id) ON DELETE RESTRICT,
    correct_language_id UUID NOT NULL REFERENCES languages (id) ON DELETE RESTRICT,
    CONSTRAINT daily_quiz_questions_position_chk CHECK (position BETWEEN 1 AND 5),
    UNIQUE (daily_quiz_id, position),
    UNIQUE (daily_quiz_id, text_id),
    UNIQUE (daily_quiz_id, correct_language_id)
);

CREATE INDEX daily_quiz_questions_daily_quiz_id_idx ON daily_quiz_questions (daily_quiz_id);
CREATE INDEX daily_quiz_questions_text_id_idx ON daily_quiz_questions (text_id);
CREATE INDEX daily_quiz_questions_correct_language_id_idx ON daily_quiz_questions (correct_language_id);

CREATE TABLE daily_quiz_options (
    question_id UUID NOT NULL REFERENCES daily_quiz_questions (id) ON DELETE CASCADE,
    language_id UUID NOT NULL REFERENCES languages (id) ON DELETE RESTRICT,
    position INTEGER NOT NULL,
    is_correct BOOLEAN NOT NULL,
    weight_at_generation INTEGER NOT NULL,
    PRIMARY KEY (question_id, language_id),
    CONSTRAINT daily_quiz_options_position_chk CHECK (position BETWEEN 1 AND 5),
    CONSTRAINT daily_quiz_options_weight_chk CHECK (weight_at_generation >= 0),
    UNIQUE (question_id, position)
);

CREATE UNIQUE INDEX daily_quiz_options_one_correct_idx ON daily_quiz_options (question_id) WHERE is_correct;
CREATE INDEX daily_quiz_options_language_id_idx ON daily_quiz_options (language_id);

CREATE TABLE anonymous_devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    preferred_locale TEXT,
    CONSTRAINT anonymous_devices_preferred_locale_chk CHECK (
        preferred_locale IS NULL OR preferred_locale IN ('pl-PL', 'en-US', 'es-ES')
    )
);

CREATE INDEX anonymous_devices_last_seen_at_idx ON anonymous_devices (last_seen_at);

CREATE TABLE quiz_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID NOT NULL REFERENCES anonymous_devices (id) ON DELETE CASCADE,
    daily_quiz_id UUID NOT NULL REFERENCES daily_quizzes (id) ON DELETE CASCADE,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    score INTEGER,
    CONSTRAINT quiz_attempts_score_chk CHECK (score IS NULL OR score BETWEEN 0 AND 5)
);

CREATE INDEX quiz_attempts_device_id_idx ON quiz_attempts (device_id);
CREATE INDEX quiz_attempts_daily_quiz_id_idx ON quiz_attempts (daily_quiz_id);
CREATE UNIQUE INDEX quiz_attempts_one_completed_idx ON quiz_attempts (device_id, daily_quiz_id) WHERE completed_at IS NOT NULL;

CREATE TABLE quiz_answers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    attempt_id UUID NOT NULL REFERENCES quiz_attempts (id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES daily_quiz_questions (id) ON DELETE RESTRICT,
    selected_language_id UUID NOT NULL REFERENCES languages (id) ON DELETE RESTRICT,
    is_correct BOOLEAN NOT NULL,
    answered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    response_time_ms INTEGER,
    CONSTRAINT quiz_answers_response_time_ms_chk CHECK (response_time_ms IS NULL OR response_time_ms >= 0),
    UNIQUE (attempt_id, question_id),
    FOREIGN KEY (question_id, selected_language_id)
        REFERENCES daily_quiz_options (question_id, language_id)
        ON DELETE RESTRICT
);

CREATE INDEX quiz_answers_attempt_id_idx ON quiz_answers (attempt_id);
CREATE INDEX quiz_answers_question_id_idx ON quiz_answers (question_id);
CREATE INDEX quiz_answers_selected_language_id_idx ON quiz_answers (selected_language_id);

-- +goose Down
DROP TABLE quiz_answers;
DROP TABLE quiz_attempts;
DROP TABLE anonymous_devices;
DROP TABLE daily_quiz_options;
DROP TABLE daily_quiz_questions;
DROP TABLE daily_quizzes;
DROP TABLE language_texts;
DROP TABLE language_names;
DROP TABLE languages;
DROP TABLE scripts;
DROP TABLE continents;
DROP TABLE language_subgroups;
DROP TABLE language_groups;
DROP TABLE language_families;
