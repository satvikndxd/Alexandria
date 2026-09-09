-- Alexandria · Migration 0012
-- Credential flags for WebAuthn records.
--
-- Migration 0002 stored transports and sign count but not the authenticator
-- flags captured at ceremony time. go-webauthn validates assertions against the
-- stored credential record, and user-presence / user-verification state is part
-- of what makes an assertion trustworthy, so it belongs in the record rather
-- than being assumed at load time.

ALTER TABLE webauthn_credentials
  ADD COLUMN flags jsonb NOT NULL DEFAULT '{"user_present": true, "user_verified": true}';

-- A credential that has never been used is indistinguishable from one whose
-- last use we failed to record; make the distinction explicit for audits.
ALTER TABLE webauthn_credentials
  ADD COLUMN clone_detected boolean NOT NULL DEFAULT false;
