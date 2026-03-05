# QA / UAT Test Plan

## Critical Host Scenarios
- Register/login/logout (password)
- Google login
- Create event, edit event, delete event
- Generate invite codes and copy
- Participant deposit appears in host list
- Approve and reject with reason

## Critical Participant Scenarios
- Open checkout by slug
- Validate invite code
- Wallet payment success path
- Manual transfer verify path
- Recheck path for delayed backend confirmation
- Rejected status shown with reason

## Edge Cases
- Invalid invite code
- Reused invite code after max usage
- Expired invite code
- Invalid tx signature
- Signature for wrong amount/recipient/mint
- Backend restart during pending payment
- Mobile UI at narrow widths

## Regression Checklist
- Auth-protected endpoints reject unauthenticated requests
- Desktop drawer and mobile modal animations
- Search/filter/sort/pagination in event list
- Search/pagination in invites/deposits lists
